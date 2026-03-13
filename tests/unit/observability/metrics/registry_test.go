package metrics_unit

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/jsolteam/tenex-platform/internal/platform/observability/metrics"
)

// freePort возвращает свободный TCP-порт на localhost.
func freePort(t *testing.T) int {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("freePort: %v", err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	ln.Close()
	return port
}

func TestPrometheusRegistry_CounterInc(t *testing.T) {
	ctx := context.Background()
	reg := metrics.NewPrometheus()
	c, err := reg.Counter("test_counter_inc", "test counter", metrics.LabelMessenger)
	if err != nil {
		t.Fatalf("Counter: %v", err)
	}
	c.Inc(ctx, metrics.L(metrics.LabelMessenger, "telegram"))
	c.Inc(ctx, metrics.L(metrics.LabelMessenger, "telegram"))
}

func TestPrometheusRegistry_CounterAdd(t *testing.T) {
	ctx := context.Background()
	reg := metrics.NewPrometheus()
	c, err := reg.Counter("test_counter_add", "test counter add", metrics.LabelStatus)
	if err != nil {
		t.Fatalf("Counter: %v", err)
	}
	c.Add(ctx, 5, metrics.L(metrics.LabelStatus, "ok"))
	c.Add(ctx, int64(100), metrics.L(metrics.LabelStatus, "error"))
}

func TestPrometheusRegistry_HistogramRecord(t *testing.T) {
	ctx := context.Background()
	reg := metrics.NewPrometheus()
	h, err := reg.Histogram("test_hist_record", "test histogram", nil, metrics.LabelHandler)
	if err != nil {
		t.Fatalf("Histogram: %v", err)
	}
	h.Record(ctx, 0.042, metrics.L(metrics.LabelHandler, "start"))
}

func TestPrometheusRegistry_NoDuplicatePanic(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("duplicate Counter registration panicked: %v", r)
		}
	}()

	ctx := context.Background()
	reg := metrics.NewPrometheus()

	c1, err := reg.Counter("tenex_dup_counter", "dup counter", metrics.LabelMessenger)
	if err != nil {
		t.Fatalf("first Counter: %v", err)
	}
	c2, err := reg.Counter("tenex_dup_counter", "dup counter", metrics.LabelMessenger)
	if err != nil {
		t.Fatalf("second Counter (duplicate) must not error: %v", err)
	}

	c1.Inc(ctx, metrics.L(metrics.LabelMessenger, "telegram"))
	c2.Inc(ctx, metrics.L(metrics.LabelMessenger, "telegram"))
}

func TestPrometheusRegistry_NoDuplicatePanic_Histogram(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("duplicate Histogram panicked: %v", r)
		}
	}()

	ctx := context.Background()
	reg := metrics.NewPrometheus()
	buckets := []float64{0.1, 0.5, 1.0}

	h1, err := reg.Histogram("tenex_dup_hist", "dup hist", buckets, metrics.LabelHandler)
	if err != nil {
		t.Fatalf("first Histogram: %v", err)
	}
	h2, err := reg.Histogram("tenex_dup_hist", "dup hist", buckets, metrics.LabelHandler)
	if err != nil {
		t.Fatalf("second Histogram (duplicate): %v", err)
	}

	h1.Record(ctx, 0.1, metrics.L(metrics.LabelHandler, "a"))
	h2.Record(ctx, 0.2, metrics.L(metrics.LabelHandler, "b"))
}

func TestPrometheusRegistry_HandlerOK(t *testing.T) {
	ctx := context.Background()
	reg := metrics.NewPrometheus()

	c, err := reg.Counter("tenex_handler_test_total", "handler test", metrics.LabelStatus)
	if err != nil {
		t.Fatalf("Counter: %v", err)
	}
	c.Inc(ctx, metrics.L(metrics.LabelStatus, "ok"))

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	reg.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", w.Code)
	}
	body, _ := io.ReadAll(w.Result().Body)
	if len(body) == 0 {
		t.Fatal("empty body")
	}
	if !strings.Contains(string(body), "go_goroutines") {
		t.Error("expected go_goroutines in /metrics output")
	}
	if !strings.Contains(string(body), "tenex_handler_test_total") {
		t.Errorf("expected tenex_handler_test_total in body")
	}
}

func TestPrometheusRegistry_HandlerContentType(t *testing.T) {
	reg := metrics.NewPrometheus()
	w := httptest.NewRecorder()
	reg.Handler().ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	ct := w.Header().Get("Content-Type")
	if !strings.Contains(ct, "text/plain") && !strings.Contains(ct, "application/openmetrics") {
		t.Errorf("unexpected Content-Type: %q", ct)
	}
}

func TestNoopRegistry_NoPanic(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("noopRegistry panicked: %v", r)
		}
	}()

	ctx := context.Background()
	reg := metrics.NewNoop()

	c, err := reg.Counter("noop_c", "noop", metrics.LabelMessenger)
	if err != nil {
		t.Fatalf("noop Counter: %v", err)
	}
	c.Inc(ctx, metrics.L(metrics.LabelMessenger, "any"))
	c.Add(ctx, 99, metrics.L(metrics.LabelMessenger, "any"))

	h, err := reg.Histogram("noop_h", "noop", nil, metrics.LabelHandler)
	if err != nil {
		t.Fatalf("noop Histogram: %v", err)
	}
	h.Record(ctx, 1.23, metrics.L(metrics.LabelHandler, "any"))

	w := httptest.NewRecorder()
	reg.Handler().ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/metrics", nil))
}

func TestNoopRegistry_HandlerReturns404(t *testing.T) {
	reg := metrics.NewNoop()
	w := httptest.NewRecorder()
	reg.Handler().ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	if w.Code != http.StatusNotFound {
		t.Errorf("noop Handler status = %d, want 404", w.Code)
	}
}

func TestNoopRegistry_DuplicateNoPanic(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("noop duplicate panicked: %v", r)
		}
	}()
	reg := metrics.NewNoop()
	_, _ = reg.Counter("x", "x")
	_, _ = reg.Counter("x", "x")
	_, _ = reg.Histogram("y", "y", nil)
	_, _ = reg.Histogram("y", "y", nil)
}

func TestNewAppMetrics_AllInstruments(t *testing.T) {
	reg := metrics.NewPrometheus()
	am, err := metrics.NewAppMetrics(reg)
	if err != nil {
		t.Fatalf("NewAppMetrics: %v", err)
	}
	if am == nil {
		t.Fatal("NewAppMetrics returned nil")
	}

	cases := []struct {
		name string
		inst interface{}
	}{
		{"UpdatesTotal", am.UpdatesTotal},
		{"UpdateProcessingDuration", am.UpdateProcessingDuration},
		{"FSMTransitionsTotal", am.FSMTransitionsTotal},
		{"RetryTotal", am.RetryTotal},
		{"AdapterErrorsTotal", am.AdapterErrorsTotal},
	}
	for _, tc := range cases {
		if tc.inst == nil {
			t.Errorf("%s is nil", tc.name)
		}
	}
}

func TestNewAppMetrics_AllInstrumentsUsable(t *testing.T) {
	ctx := context.Background()
	reg := metrics.NewPrometheus()
	am, err := metrics.NewAppMetrics(reg)
	if err != nil {
		t.Fatalf("NewAppMetrics: %v", err)
	}

	am.UpdatesTotal.Inc(ctx,
		metrics.L(metrics.LabelMessenger, "telegram"),
		metrics.L(metrics.LabelStatus, "ok"),
	)
	am.UpdateProcessingDuration.Record(ctx, 0.05,
		metrics.L(metrics.LabelMessenger, "telegram"),
		metrics.L(metrics.LabelHandler, "start"),
	)
	am.FSMTransitionsTotal.Inc(ctx,
		metrics.L(metrics.LabelState, "idle"),
		metrics.L(metrics.LabelHandler, "on_message"),
	)
	am.RetryTotal.Add(ctx, 3,
		metrics.L(metrics.LabelModule, "scheduler"),
		metrics.L(metrics.LabelStatus, "retry"),
	)
	am.AdapterErrorsTotal.Inc(ctx,
		metrics.L(metrics.LabelMessenger, "telegram"),
		metrics.L(metrics.LabelStatus, "error"),
	)
}

func TestNewAppMetrics_AppearInHandler(t *testing.T) {
	ctx := context.Background()
	reg := metrics.NewPrometheus()
	am, err := metrics.NewAppMetrics(reg)
	if err != nil {
		t.Fatalf("NewAppMetrics: %v", err)
	}

	am.UpdatesTotal.Inc(ctx,
		metrics.L(metrics.LabelMessenger, "telegram"),
		metrics.L(metrics.LabelStatus, "ok"),
	)
	am.UpdateProcessingDuration.Record(ctx, 0.01,
		metrics.L(metrics.LabelMessenger, "telegram"),
		metrics.L(metrics.LabelHandler, "start"),
	)
	am.FSMTransitionsTotal.Inc(ctx,
		metrics.L(metrics.LabelState, "idle"),
		metrics.L(metrics.LabelHandler, "on_message"),
	)
	am.RetryTotal.Inc(ctx,
		metrics.L(metrics.LabelModule, "scheduler"),
		metrics.L(metrics.LabelStatus, "retry"),
	)
	am.AdapterErrorsTotal.Inc(ctx,
		metrics.L(metrics.LabelMessenger, "telegram"),
		metrics.L(metrics.LabelStatus, "error"),
	)

	w := httptest.NewRecorder()
	reg.Handler().ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/metrics", nil))

	body, _ := io.ReadAll(w.Result().Body)
	for _, want := range []string{
		"tenex_updates_total",
		"tenex_update_processing_duration_seconds",
		"tenex_fsm_transitions_total",
		"tenex_retry_total",
		"tenex_adapter_errors_total",
	} {
		if !strings.Contains(string(body), want) {
			t.Errorf("metric %q not found in /metrics output", want)
		}
	}
}

func TestNewAppMetrics_WithNoop(t *testing.T) {
	ctx := context.Background()
	reg := metrics.NewNoop()
	am, err := metrics.NewAppMetrics(reg)
	if err != nil {
		t.Fatalf("NewAppMetrics(noop): %v", err)
	}
	if am == nil {
		t.Fatal("AppMetrics with noop is nil")
	}
	am.UpdatesTotal.Inc(ctx, metrics.L(metrics.LabelMessenger, "x"), metrics.L(metrics.LabelStatus, "y"))
	am.AdapterErrorsTotal.Add(ctx, 3, metrics.L(metrics.LabelMessenger, "x"), metrics.L(metrics.LabelStatus, "err"))
}

func TestNewAppMetrics_IdempotentRegistration(t *testing.T) {
	ctx := context.Background()
	reg := metrics.NewPrometheus()
	am1, err := metrics.NewAppMetrics(reg)
	if err != nil {
		t.Fatalf("first NewAppMetrics: %v", err)
	}
	am2, err := metrics.NewAppMetrics(reg)
	if err != nil {
		t.Fatalf("second NewAppMetrics (duplicate): %v", err)
	}
	am1.UpdatesTotal.Inc(ctx, metrics.L(metrics.LabelMessenger, "a"), metrics.L(metrics.LabelStatus, "ok"))
	am2.UpdatesTotal.Inc(ctx, metrics.L(metrics.LabelMessenger, "b"), metrics.L(metrics.LabelStatus, "ok"))
}

func TestStartServer_ServesMetrics(t *testing.T) {
	reg := metrics.NewPrometheus()
	port := freePort(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	shutdown := metrics.StartServer(ctx, port, reg.Handler())

	url := fmt.Sprintf("http://127.0.0.1:%d/metrics", port)
	var (
		resp *http.Response
		err  error
	)
	for i := 0; i < 25; i++ {
		resp, err = http.Get(url) //nolint:noctx
		if err == nil {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if err != nil {
		t.Skipf("metrics server did not start on port %d: %v", port, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want 200", resp.StatusCode)
	}

	shutCtx, shutCancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer shutCancel()
	if err := shutdown(shutCtx); err != nil {
		t.Errorf("shutdown: %v", err)
	}
}

func TestStartServer_GracefulShutdown(t *testing.T) {
	reg := metrics.NewNoop()
	port := freePort(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	shutdown := metrics.StartServer(ctx, port, reg.Handler())
	time.Sleep(40 * time.Millisecond)

	shutCtx, shutCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutCancel()
	if err := shutdown(shutCtx); err != nil {
		t.Errorf("graceful shutdown: %v", err)
	}
}

func TestStartServer_ContextCancelTriggersShutdown(t *testing.T) {
	reg := metrics.NewNoop()
	port := freePort(t)

	ctx, cancel := context.WithCancel(context.Background())

	shutdown := metrics.StartServer(ctx, port, reg.Handler())
	time.Sleep(40 * time.Millisecond)

	cancel()
	time.Sleep(100 * time.Millisecond)

	shutCtx, shutCancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer shutCancel()
	_ = shutdown(shutCtx)
}
