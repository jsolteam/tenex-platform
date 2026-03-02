package container_unit

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/jsol/tenex-platform/internal/platform/container"
)

type recorder struct {
	mu  sync.Mutex
	log []string
}

func (r *recorder) append(s string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.log = append(r.log, s)
}

func (r *recorder) snapshot() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	cp := make([]string, len(r.log))
	copy(cp, r.log)
	return cp
}

type trackedComponent struct {
	name     string
	rec      *recorder
	startErr error
	stopErr  error
}

func (c *trackedComponent) Start(_ context.Context) error {
	if c.startErr != nil {
		return c.startErr
	}
	c.rec.append("start:" + c.name)
	return nil
}

func (c *trackedComponent) Stop(_ context.Context) error {
	c.rec.append("stop:" + c.name)
	return c.stopErr
}

func newTracked(name string, rec *recorder) *trackedComponent {
	return &trackedComponent{name: name, rec: rec}
}

func newTrackedFailing(name string, rec *recorder, startErr, stopErr error) *trackedComponent {
	return &trackedComponent{name: name, rec: rec, startErr: startErr, stopErr: stopErr}
}

func TestContainer_StartOrder(t *testing.T) {
	rec := &recorder{}
	c := container.New()
	c.Register("alpha", newTracked("alpha", rec))
	c.Register("beta", newTracked("beta", rec))
	c.Register("gamma", newTracked("gamma", rec))

	if err := c.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}

	got := rec.snapshot()
	want := []string{"start:alpha", "start:beta", "start:gamma"}

	if len(got) != len(want) {
		t.Fatalf("want %v, got %v", want, got)
	}
	for i, w := range want {
		if got[i] != w {
			t.Errorf("event[%d]: want %q, got %q", i, w, got[i])
		}
	}
}

func TestContainer_StopReverseOrder(t *testing.T) {
	rec := &recorder{}
	c := container.New()
	c.Register("alpha", newTracked("alpha", rec))
	c.Register("beta", newTracked("beta", rec))
	c.Register("gamma", newTracked("gamma", rec))

	if err := c.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}

	rec.mu.Lock()
	rec.log = nil
	rec.mu.Unlock()

	if err := c.Stop(context.Background()); err != nil {
		t.Fatalf("Stop: %v", err)
	}

	got := rec.snapshot()
	want := []string{"stop:gamma", "stop:beta", "stop:alpha"}

	if len(got) != len(want) {
		t.Fatalf("want %v, got %v", want, got)
	}
	for i, w := range want {
		if got[i] != w {
			t.Errorf("event[%d]: want %q, got %q", i, w, got[i])
		}
	}
}

func TestContainer_StartFailureRollback(t *testing.T) {
	rec := &recorder{}
	c := container.New()
	c.Register("alpha", newTracked("alpha", rec))
	c.Register("beta", newTracked("beta", rec))
	c.Register("gamma", newTrackedFailing("gamma", rec, errors.New("gamma boom"), nil))
	c.Register("delta", newTracked("delta", rec))

	err := c.Start(context.Background())
	if err == nil {
		t.Fatal("Start should have returned error")
	}
	if !strings.Contains(err.Error(), "gamma") {
		t.Errorf("error should mention failing component 'gamma': %v", err)
	}

	got := rec.snapshot()
	want := []string{"start:alpha", "start:beta", "stop:beta", "stop:alpha"}

	if len(got) != len(want) {
		t.Fatalf("rollback events: want %v, got %v", want, got)
	}
	for i, w := range want {
		if got[i] != w {
			t.Errorf("event[%d]: want %q, got %q", i, w, got[i])
		}
	}
}

func TestContainer_StartFailureRollback_FirstComponent(t *testing.T) {
	rec := &recorder{}
	c := container.New()
	c.Register("alpha", newTrackedFailing("alpha", rec, errors.New("alpha boom"), nil))
	c.Register("beta", newTracked("beta", rec))

	if err := c.Start(context.Background()); err == nil {
		t.Fatal("Start should have returned error")
	}

	got := rec.snapshot()
	if len(got) != 0 {
		t.Errorf("expected no events, got %v", got)
	}
}

func TestContainer_StopContinuesOnError(t *testing.T) {
	rec := &recorder{}
	c := container.New()
	c.Register("alpha", newTracked("alpha", rec))
	c.Register("beta", newTrackedFailing("beta", rec, nil, errors.New("beta stop boom")))
	c.Register("gamma", newTracked("gamma", rec))

	if err := c.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}

	rec.mu.Lock()
	rec.log = nil
	rec.mu.Unlock()

	err := c.Stop(context.Background())
	if err == nil {
		t.Fatal("Stop should have returned error from beta")
	}
	if !strings.Contains(err.Error(), "beta") {
		t.Errorf("error should mention 'beta': %v", err)
	}

	got := rec.snapshot()
	want := []string{"stop:gamma", "stop:beta", "stop:alpha"}

	if len(got) != len(want) {
		t.Fatalf("stop events: want %v, got %v", want, got)
	}
	for i, w := range want {
		if got[i] != w {
			t.Errorf("event[%d]: want %q, got %q", i, w, got[i])
		}
	}
}

func TestContainer_StopMultipleErrors(t *testing.T) {
	rec := &recorder{}
	c := container.New()
	c.Register("alpha", newTrackedFailing("alpha", rec, nil, errors.New("alpha stop err")))
	c.Register("beta", newTracked("beta", rec))
	c.Register("gamma", newTrackedFailing("gamma", rec, nil, errors.New("gamma stop err")))

	if err := c.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}

	err := c.Stop(context.Background())
	if err == nil {
		t.Fatal("Stop should return error")
	}
	if !strings.Contains(err.Error(), "alpha") {
		t.Errorf("error missing 'alpha': %v", err)
	}
	if !strings.Contains(err.Error(), "gamma") {
		t.Errorf("error missing 'gamma': %v", err)
	}
}

func TestContainer_EmptyStart(t *testing.T) {
	c := container.New()
	if err := c.Start(context.Background()); err != nil {
		t.Fatalf("Start on empty container: %v", err)
	}
	if err := c.Stop(context.Background()); err != nil {
		t.Fatalf("Stop on empty container: %v", err)
	}
}

func TestContainer_StopBeforeStart(t *testing.T) {
	rec := &recorder{}
	c := container.New()
	c.Register("alpha", newTracked("alpha", rec))

	if err := c.Stop(context.Background()); err != nil {
		t.Fatalf("Stop before Start: %v", err)
	}
	if len(rec.snapshot()) != 0 {
		t.Errorf("expected no events, got %v", rec.snapshot())
	}
}

func TestContainer_StartOrder_Many(t *testing.T) {
	const n = 10
	rec := &recorder{}
	c := container.New()
	for i := 0; i < n; i++ {
		c.Register(fmt.Sprintf("comp%d", i), newTracked(fmt.Sprintf("comp%d", i), rec))
	}

	if err := c.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}

	got := rec.snapshot()
	for i := 0; i < n; i++ {
		want := fmt.Sprintf("start:comp%d", i)
		if got[i] != want {
			t.Errorf("event[%d]: want %q, got %q", i, want, got[i])
		}
	}

	rec.mu.Lock()
	rec.log = nil
	rec.mu.Unlock()

	if err := c.Stop(context.Background()); err != nil {
		t.Fatalf("Stop: %v", err)
	}

	got = rec.snapshot()
	for i := 0; i < n; i++ {
		want := fmt.Sprintf("stop:comp%d", n-1-i)
		if got[i] != want {
			t.Errorf("stop event[%d]: want %q, got %q", i, want, got[i])
		}
	}
}

func TestContainer_ContextPropagated(t *testing.T) {
	type contextKey struct{}
	ctx := context.WithValue(context.Background(), contextKey{}, "marker")

	startGot := ""
	stopGot := ""

	comp := &contextCheckComponent{
		onStart: func(c context.Context) { startGot, _ = c.Value(contextKey{}).(string) },
		onStop:  func(c context.Context) { stopGot, _ = c.Value(contextKey{}).(string) },
	}

	c := container.New()
	c.Register("ctx-check", comp)

	_ = c.Start(ctx)
	_ = c.Stop(ctx)

	if startGot != "marker" {
		t.Errorf("Start context not propagated: got %q", startGot)
	}
	if stopGot != "marker" {
		t.Errorf("Stop context not propagated: got %q", stopGot)
	}
}

type contextCheckComponent struct {
	onStart func(context.Context)
	onStop  func(context.Context)
}

func (c *contextCheckComponent) Start(ctx context.Context) error { c.onStart(ctx); return nil }
func (c *contextCheckComponent) Stop(ctx context.Context) error  { c.onStop(ctx); return nil }
