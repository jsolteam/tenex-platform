package shutdown_unit

import (
	"context"
	"errors"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jsol/tenex-platform/internal/platform/shutdown"
)

func noop(_ context.Context) error { return nil }

func failHook(msg string) func(context.Context) error {
	return func(_ context.Context) error { return errors.New(msg) }
}

func recordHook(mu *sync.Mutex, log *[]string, name string) func(context.Context) error {
	return func(_ context.Context) error {
		mu.Lock()
		defer mu.Unlock()
		*log = append(*log, name)
		return nil
	}
}

func TestManager_Stop_RunsHooksOnce(t *testing.T) {
	sm := shutdown.New(5 * time.Second)

	var count atomic.Int64
	sm.Register("a", func(_ context.Context) error { count.Add(1); return nil })
	sm.Register("b", func(_ context.Context) error { count.Add(1); return nil })

	if err := sm.Stop(); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	if count.Load() != 2 {
		t.Errorf("hooks called %d times, want 2", count.Load())
	}
}

func TestManager_Stop_Idempotent(t *testing.T) {
	sm := shutdown.New(5 * time.Second)

	var count atomic.Int64
	sm.Register("only", func(_ context.Context) error { count.Add(1); return nil })

	_ = sm.Stop()
	_ = sm.Stop()

	if count.Load() != 1 {
		t.Errorf("hook called %d times, want 1", count.Load())
	}
}

func TestManager_Stop_ConcurrentIdempotent(t *testing.T) {
	sm := shutdown.New(5 * time.Second)

	var count atomic.Int64
	sm.Register("concurrent", func(_ context.Context) error {
		count.Add(1)
		time.Sleep(10 * time.Millisecond)
		return nil
	})

	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = sm.Stop()
		}()
	}
	wg.Wait()

	if count.Load() != 1 {
		t.Errorf("hook called %d times, want exactly 1", count.Load())
	}
}

func TestManager_Stop_OrderPreserved(t *testing.T) {
	sm := shutdown.New(5 * time.Second)

	var mu sync.Mutex
	var log []string
	sm.Register("updates", recordHook(&mu, &log, "updates"))
	sm.Register("workers", recordHook(&mu, &log, "workers"))
	sm.Register("logs", recordHook(&mu, &log, "logs"))
	sm.Register("tracer", recordHook(&mu, &log, "tracer"))
	sm.Register("redis", recordHook(&mu, &log, "redis"))
	sm.Register("db", recordHook(&mu, &log, "db"))

	if err := sm.Stop(); err != nil {
		t.Fatalf("Stop: %v", err)
	}

	want := []string{"updates", "workers", "logs", "tracer", "redis", "db"}
	if len(log) != len(want) {
		t.Fatalf("got %v, want %v", log, want)
	}
	for i, w := range want {
		if log[i] != w {
			t.Errorf("hook[%d]: got %q, want %q", i, log[i], w)
		}
	}
}

func TestManager_Stop_ContinuesOnError(t *testing.T) {
	sm := shutdown.New(5 * time.Second)

	var mu sync.Mutex
	var log []string
	sm.Register("a", recordHook(&mu, &log, "a"))
	sm.Register("b", failHook("b exploded"))
	sm.Register("c", recordHook(&mu, &log, "c"))

	err := sm.Stop()
	if err == nil {
		t.Fatal("Stop should return error from hook b")
	}
	if !strings.Contains(err.Error(), "b exploded") {
		t.Errorf("error should mention 'b exploded': %v", err)
	}
	if len(log) != 2 || log[0] != "a" || log[1] != "c" {
		t.Errorf("expected [a c], got %v", log)
	}
}

func TestManager_Stop_MultipleErrors(t *testing.T) {
	sm := shutdown.New(5 * time.Second)
	sm.Register("x", failHook("err-x"))
	sm.Register("y", noop)
	sm.Register("z", failHook("err-z"))

	err := sm.Stop()
	if err == nil {
		t.Fatal("expected joined error")
	}
	if !strings.Contains(err.Error(), "err-x") {
		t.Errorf("missing err-x: %v", err)
	}
	if !strings.Contains(err.Error(), "err-z") {
		t.Errorf("missing err-z: %v", err)
	}
}

func TestManager_Stop_PanicRecovery(t *testing.T) {
	sm := shutdown.New(5 * time.Second)

	var afterPanic atomic.Bool
	sm.Register("panic-hook", func(_ context.Context) error { panic("intentional") })
	sm.Register("after-panic", func(_ context.Context) error {
		afterPanic.Store(true)
		return nil
	})

	err := sm.Stop()
	if err == nil {
		t.Fatal("Stop should return error from panicking hook")
	}
	if !strings.Contains(err.Error(), "panicked") {
		t.Errorf("error should mention 'panicked': %v", err)
	}
	if !afterPanic.Load() {
		t.Error("hook after panicking hook was not executed")
	}
}

func TestManager_Stop_Timeout(t *testing.T) {
	const tout = 80 * time.Millisecond
	sm := shutdown.New(tout)

	var second atomic.Bool
	sm.Register("slow", func(ctx context.Context) error {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(10 * time.Second):
			return nil
		}
	})
	sm.Register("skipped", func(_ context.Context) error {
		second.Store(true)
		return nil
	})

	start := time.Now()
	err := sm.Stop()
	elapsed := time.Since(start)

	if elapsed > 5*tout {
		t.Errorf("Stop took %v, expected ~%v", elapsed, tout)
	}
	if err == nil {
		t.Fatal("expected timeout-related error")
	}
}

func TestManager_Stop_EmptyHooks(t *testing.T) {
	sm := shutdown.New(5 * time.Second)
	if err := sm.Stop(); err != nil {
		t.Fatalf("Stop with no hooks: %v", err)
	}
}

func TestManager_Stop_ContextPassedToHook(t *testing.T) {
	sm := shutdown.New(5 * time.Second)

	var hookCtx context.Context
	sm.Register("ctx-check", func(ctx context.Context) error {
		hookCtx = ctx
		return nil
	})

	if err := sm.Stop(); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	if hookCtx == nil {
		t.Fatal("hook received nil context")
	}
	_, ok := hookCtx.Deadline()
	if !ok {
		t.Fatal("hook context has no deadline")
	}
}

func TestManager_Wait_StopTriggersIt(t *testing.T) {
	sm := shutdown.New(5 * time.Second)

	var ran atomic.Bool
	sm.Register("hook", func(_ context.Context) error {
		ran.Store(true)
		return nil
	})

	done := make(chan error, 1)
	go func() { done <- sm.Wait(context.Background()) }()

	time.Sleep(20 * time.Millisecond)
	_ = sm.Stop()

	select {
	case err := <-done:
		if err != nil {
			t.Errorf("Wait: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("Wait did not return after Stop()")
	}
	if !ran.Load() {
		t.Error("hook was not called")
	}
}

func TestManager_Wait_ContextCancelTriggersIt(t *testing.T) {
	sm := shutdown.New(5 * time.Second)

	var ran atomic.Bool
	sm.Register("hook", func(_ context.Context) error {
		ran.Store(true)
		return nil
	})

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- sm.Wait(ctx) }()

	time.Sleep(20 * time.Millisecond)
	cancel()

	select {
	case err := <-done:
		if err != nil {
			t.Errorf("Wait: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("Wait did not return after context cancel")
	}
	if !ran.Load() {
		t.Error("hook was not called")
	}
}

func TestManager_WaitAndStop_Concurrent(t *testing.T) {
	sm := shutdown.New(5 * time.Second)

	var count atomic.Int64
	sm.Register("race", func(_ context.Context) error {
		count.Add(1)
		return nil
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		_ = sm.Wait(ctx)
	}()
	wg.Add(1)
	go func() {
		defer wg.Done()
		time.Sleep(5 * time.Millisecond)
		_ = sm.Stop()
	}()
	wg.Wait()

	if count.Load() != 1 {
		t.Errorf("hook ran %d times, want 1", count.Load())
	}
}

func TestManager_GracefulShutdown_CanonicalOrder(t *testing.T) {
	sm := shutdown.New(10 * time.Second)

	var mu sync.Mutex
	var log []string
	rec := func(name string) func(context.Context) error {
		return recordHook(&mu, &log, name)
	}

	sm.Register("updates", rec("updates"))
	sm.Register("workers", rec("workers"))
	sm.Register("logs", rec("logs"))
	sm.Register("tracer", rec("tracer"))
	sm.Register("redis", rec("redis"))
	sm.Register("db", rec("db"))

	if err := sm.Stop(); err != nil {
		t.Fatalf("Stop: %v", err)
	}

	want := []string{"updates", "workers", "logs", "tracer", "redis", "db"}
	if len(log) != len(want) {
		t.Fatalf("want %v, got %v", want, log)
	}
	for i, w := range want {
		if log[i] != w {
			t.Errorf("step %d: want %q, got %q", i+1, w, log[i])
		}
	}
}

func TestManager_NoGoroutineLeak(t *testing.T) {
	sm := shutdown.New(5 * time.Second)
	sm.Register("fast", func(_ context.Context) error {
		time.Sleep(5 * time.Millisecond)
		return nil
	})

	done := make(chan struct{})
	go func() {
		_ = sm.Stop()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("Stop did not return — goroutine leak suspected")
	}
}
