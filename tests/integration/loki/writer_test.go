package loki_integration

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jsol/tenex-platform/internal/platform/logger/exporters/loki"
)

type fakeLoki struct {
	mu       sync.Mutex
	requests []pushRequest
	status   int
	srv      *httptest.Server
}

type pushRequest struct {
	body map[string]interface{}
}

func newFakeLoki(status int) *fakeLoki {
	fl := &fakeLoki{status: status}
	fl.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		var body map[string]interface{}
		_ = json.Unmarshal(raw, &body)
		fl.mu.Lock()
		fl.requests = append(fl.requests, pushRequest{body: body})
		fl.mu.Unlock()
		w.WriteHeader(fl.status)
	}))
	return fl
}

func (f *fakeLoki) URL() string { return f.srv.URL }
func (f *fakeLoki) Close()      { f.srv.Close() }
func (f *fakeLoki) count() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.requests)
}
func (f *fakeLoki) last() map[string]interface{} {
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.requests) == 0 {
		return nil
	}
	return f.requests[len(f.requests)-1].body
}

// TestWriter_FlushSendsToLoki — запись + flush → HTTP POST в Loki.
func TestWriter_FlushSendsToLoki(t *testing.T) {
	srv := newFakeLoki(http.StatusNoContent)
	defer srv.Close()

	w := loki.NewWriter(srv.URL(), loki.Labels{"app": "test"})
	stop := make(chan struct{})
	go w.Run(30*time.Millisecond, stop)

	if _, err := w.Write([]byte(`{"msg":"hello"}`)); err != nil {
		t.Fatalf("Write: %v", err)
	}

	waitFor(t, 500*time.Millisecond, func() bool { return srv.count() > 0 })
	close(stop)

	if srv.count() == 0 {
		t.Fatal("no requests sent to Loki after flush")
	}
}

// TestWriter_CorrectPayloadStructure — тело запроса соответствует Loki API.
func TestWriter_CorrectPayloadStructure(t *testing.T) {
	srv := newFakeLoki(http.StatusNoContent)
	defer srv.Close()

	labels := loki.Labels{"app": "myapp", "env": "test"}
	w := loki.NewWriter(srv.URL(), labels)
	stop := make(chan struct{})
	go w.Run(20*time.Millisecond, stop)

	_, _ = w.Write([]byte("log line"))
	waitFor(t, 500*time.Millisecond, func() bool { return srv.count() > 0 })
	close(stop)

	body := srv.last()
	if body == nil {
		t.Fatal("no body received")
	}

	streams, ok := body["streams"].([]interface{})
	if !ok || len(streams) == 0 {
		t.Fatal("missing or empty 'streams' in payload")
	}

	stream, _ := streams[0].(map[string]interface{})
	sentLabels, _ := stream["stream"].(map[string]interface{})

	for k, want := range labels {
		if got, ok := sentLabels[k]; !ok {
			t.Errorf("label %q missing", k)
		} else if got != want {
			t.Errorf("label %q = %q, want %q", k, got, want)
		}
	}

	values, _ := stream["values"].([]interface{})
	if len(values) == 0 {
		t.Error("no values in stream")
	}
}

// TestWriter_GracefulShutdownFlushes — при stop-сигнале Writer flush'ит буфер.
func TestWriter_GracefulShutdownFlushes(t *testing.T) {
	srv := newFakeLoki(http.StatusNoContent)
	defer srv.Close()

	w := loki.NewWriter(srv.URL(), loki.Labels{"app": "test"})
	stop := make(chan struct{})

	done := make(chan struct{})
	go func() {
		w.Run(10*time.Second, stop)
		close(done)
	}()

	for i := 0; i < 5; i++ {
		_, _ = w.Write([]byte(`{"k":"v"}`))
	}

	close(stop)
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Run() did not return after stop")
	}

	if srv.count() == 0 {
		t.Error("graceful shutdown did not flush data to Loki")
	}
}

func TestWriter_BackpressureNoPanic(t *testing.T) {
	srv := newFakeLoki(http.StatusInternalServerError)
	defer srv.Close()

	w := loki.NewWriter(srv.URL(), loki.Labels{"app": "test"})

	for i := 0; i < 11_000; i++ {
		n, err := w.Write([]byte("x"))
		if err != nil {
			t.Fatalf("Write[%d] error: %v", i, err)
		}
		if n != 1 {
			t.Fatalf("Write[%d] n=%d, want 1", i, n)
		}
	}
}

func TestWriter_RequeueOnHTTPError(t *testing.T) {
	srv := newFakeLoki(http.StatusInternalServerError)
	defer srv.Close()

	w := loki.NewWriter(srv.URL(), loki.Labels{"app": "test"})
	stop := make(chan struct{})
	go w.Run(30*time.Millisecond, stop)

	for i := 0; i < 10; i++ {
		_, _ = w.Write([]byte(`{"i":"v"}`))
	}
	time.Sleep(250 * time.Millisecond)
	close(stop)

	if srv.count() == 0 {
		t.Fatal("no retry attempts — entries silently dropped without retrying")
	}
}

// TestWriter_RngConcurrentSafety — rng защищён mutex при конкурентных flush+write.
// Запускать с -race.
func TestWriter_RngConcurrentSafety(t *testing.T) {
	var reqCount atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.ReadAll(r.Body)
		if reqCount.Add(1) < 3 {
			w.WriteHeader(http.StatusInternalServerError) // первые 2 = fail → retry + jitter
		} else {
			w.WriteHeader(http.StatusNoContent)
		}
	}))
	defer srv.Close()

	w := loki.NewWriter(srv.URL, loki.Labels{"app": "test"})

	var wg sync.WaitGroup
	stop := make(chan struct{})

	wg.Add(1)
	go func() {
		defer wg.Done()
		w.Run(10*time.Millisecond, stop)
	}()

	for i := 0; i < 30; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = w.Write([]byte("concurrent"))
		}()
	}

	time.Sleep(300 * time.Millisecond)
	close(stop)
	wg.Wait()
}

func waitFor(t *testing.T, timeout time.Duration, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
}
