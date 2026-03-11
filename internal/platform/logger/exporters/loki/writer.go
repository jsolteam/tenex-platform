package loki

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"os"
	"strconv"
	"sync"
	"sync/atomic"
	"time"
)

const (
	defaultInterval = 2 * time.Second
	maxBatchSize    = 10_000
	chunkSize       = 1_000
	maxRetries      = 3
	retryBaseDelay  = 200 * time.Millisecond
	retryMaxDelay   = 5 * time.Second
)

type Labels map[string]string

type entry struct {
	ts  time.Time
	msg string
}

type Writer struct {
	endpoint string
	labels   Labels
	client   *http.Client

	rngMu sync.Mutex
	rng   *rand.Rand

	mu           sync.Mutex
	batch        []entry
	droppedCount atomic.Int64
}

func NewWriter(endpoint string, labels Labels) *Writer {
	transport := &http.Transport{
		MaxIdleConnsPerHost: 4,
		IdleConnTimeout:     30 * time.Second,
	}
	return &Writer{
		endpoint: endpoint,
		labels:   labels,
		client:   &http.Client{Timeout: 5 * time.Second, Transport: transport},
		rng:      rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

func (w *Writer) Write(p []byte) (int, error) {
	w.mu.Lock()
	if len(w.batch) >= maxBatchSize {
		w.mu.Unlock()
		if n := w.droppedCount.Add(1); n%100 == 0 {
			fmt.Fprintf(os.Stderr, "[loki] batch full, total dropped: %d\n", n)
		}
		return len(p), nil
	}
	w.batch = append(w.batch, entry{ts: time.Now(), msg: string(p)})
	w.mu.Unlock()
	return len(p), nil
}

func (w *Writer) Run(interval time.Duration, stop <-chan struct{}) {
	if interval <= 0 {
		interval = defaultInterval
	}
	t := time.NewTicker(interval)
	defer t.Stop()

	for {
		select {
		case <-t.C:
			w.flush(stop)
		case <-stop:
			w.flush(stop)
			return
		}
	}
}

func (w *Writer) flush(stop <-chan struct{}) {
	w.mu.Lock()
	if len(w.batch) == 0 {
		w.mu.Unlock()
		return
	}
	snapshot := make([]entry, len(w.batch))
	copy(snapshot, w.batch)
	w.batch = w.batch[:0]
	w.mu.Unlock()

	var failed []entry
	for i := 0; i < len(snapshot); i += chunkSize {
		end := i + chunkSize
		if end > len(snapshot) {
			end = len(snapshot)
		}
		if err := w.pushWithRetry(snapshot[i:end], stop); err != nil {
			failed = append(failed, snapshot[i:end]...)
		}
	}

	if len(failed) == 0 {
		return
	}

	w.mu.Lock()
	capacity := maxBatchSize - len(failed)
	newEntries := w.batch
	if capacity <= 0 {
		newEntries = nil
		n := int64(-capacity) + int64(len(w.batch))
		if n > 0 {
			w.droppedCount.Add(n)
			fmt.Fprintf(os.Stderr, "[loki] %d new entries dropped while requeueing failed ones\n", n)
		}
	} else if len(newEntries) > capacity {
		dropped := int64(len(newEntries) - capacity)
		w.droppedCount.Add(dropped)
		fmt.Fprintf(os.Stderr, "[loki] %d new entries dropped while requeueing failed ones\n", dropped)
		newEntries = newEntries[:capacity]
	}

	combined := make([]entry, 0, len(failed)+len(newEntries))
	combined = append(combined, failed...)
	combined = append(combined, newEntries...)
	w.batch = combined
	w.mu.Unlock()

	fmt.Fprintf(os.Stderr, "[loki] %d entries re-queued after push failure\n", len(failed))
}

func (w *Writer) jitter(delay time.Duration) time.Duration {
	if delay/5 <= 0 {
		return 0
	}
	w.rngMu.Lock()
	n := w.rng.Int63n(int64(delay / 5))
	w.rngMu.Unlock()
	return time.Duration(n)
}

func (w *Writer) pushWithRetry(chunk []entry, stop <-chan struct{}) error {
	body := w.buildPayload(chunk)
	delay := retryBaseDelay

	var lastErr error
	for attempt := 0; attempt < maxRetries; attempt++ {
		if attempt > 0 {
			sleep := delay + w.jitter(delay)
			select {
			case <-time.After(sleep):
			case <-stop:
				return lastErr
			}
			delay *= 2
			if delay > retryMaxDelay {
				delay = retryMaxDelay
			}
		}
		if err := w.doPost(body); err == nil {
			return nil
		} else {
			lastErr = err
		}
	}
	fmt.Fprintf(os.Stderr, "[loki] push failed after %d attempts: %v\n", maxRetries, lastErr)
	return lastErr
}

func (w *Writer) doPost(body []byte) error {
	resp, err := w.client.Post(
		w.endpoint+"/loki/api/v1/push",
		"application/json",
		bytes.NewReader(body),
	)
	if err != nil {
		return err
	}
	defer func() {
		_, _ = io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
	}()

	if resp.StatusCode >= 300 {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	return nil
}

func (w *Writer) buildPayload(chunk []entry) []byte {
	values := make([][]string, 0, len(chunk))
	for _, e := range chunk {
		values = append(values, []string{
			strconv.FormatInt(e.ts.UnixNano(), 10),
			e.msg,
		})
	}
	payload := map[string]interface{}{
		"streams": []map[string]interface{}{
			{"stream": w.labels, "values": values},
		},
	}
	data, _ := json.Marshal(payload)
	return data
}
