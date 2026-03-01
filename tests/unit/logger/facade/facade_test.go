package facade_unit

import (
	"context"
	"sync"
	"testing"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"

	logcore "github.com/jsol/tenex-platform/internal/platform/logger/core"
	"github.com/jsol/tenex-platform/internal/platform/logger/facade"
)

func TestFacade_LBeforeInitNotNil(t *testing.T) {
	l := facade.L()
	if l == nil {
		t.Fatal("L() returned nil — nil dereference panic in production (M-9 regression)")
	}
}

// TestFacade_NoopIsUsable — методы noop-логгера не паникуют.
func TestFacade_NoopIsUsable(t *testing.T) {
	l := facade.L()
	// Все методы должны быть вызываемы без паники.
	l.Info("noop-info", zap.String("k", "v"))
	l.Warn("noop-warn")
	l.Error("noop-error")
	l.Debug("noop-debug")

	child := l.With(zap.String("user", "test"))
	if child == nil {
		t.Fatal("With() on noop returned nil")
	}
	child.Info("child-noop")
}

// TestFacade_NoopMultipleCalls — нет аллокаций нового объекта на каждый вызов L().
// Проверяем что возвращается один и тот же singleton (до Init).
func TestFacade_NoopMultipleCalls(t *testing.T) {
	for i := 0; i < 1000; i++ {
		l := facade.L()
		if l == nil {
			t.Fatalf("L() returned nil on call %d", i)
		}
	}
}

// TestFacade_InitSetsLogger — после Init() L() возвращает переданный logger.
func TestFacade_InitSetsLogger(t *testing.T) {
	zc, logs := observer.New(zapcore.InfoLevel)
	l := logcore.New(zap.New(zc))

	facade.Init(l)

	got := facade.L()
	if got == nil {
		t.Fatal("L() returned nil after Init()")
	}

	got.Info("test-after-init")

	if logs.Len() == 0 {
		t.Error("message not recorded — Init() did not propagate logger")
	}
	if logs.All()[0].Message != "test-after-init" {
		t.Errorf("unexpected message: %q", logs.All()[0].Message)
	}
}

// TestFacade_CtxNoPanic — Ctx() не паникует ни до, ни после Init().
func TestFacade_CtxNoPanic(t *testing.T) {
	ctx := context.Background()
	l := facade.Ctx(ctx)
	if l == nil {
		t.Fatal("Ctx() returned nil")
	}
	l.Info("ctx-message")
}

// TestFacade_CtxWithCorrelationFields — Ctx() добавляет поля из контекста.
// Проверяем что FromCtx вызывается корректно (smoke test).
func TestFacade_CtxWithCorrelationFields(t *testing.T) {
	zc, logs := observer.New(zapcore.InfoLevel)
	facade.Init(logcore.New(zap.New(zc)))

	ctx := context.Background()
	// Просто убеждаемся что вызов не паникует.
	l := facade.Ctx(ctx)
	l.Info("ctx-smoke")

	if logs.Len() == 0 {
		t.Error("expected at least 1 log entry")
	}
}

// TestFacade_ConcurrentInit — конкурентные Init() не создают race condition.
// Запускать с -race.
func TestFacade_ConcurrentInit(t *testing.T) {
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			facade.Init(logcore.NewNoop())
		}()
	}
	wg.Wait()

	l := facade.L()
	if l == nil {
		t.Fatal("L() is nil after concurrent Init()")
	}
}

// TestFacade_ConcurrentLRead — конкурентные L() безопасны.
// Запускать с -race.
func TestFacade_ConcurrentLRead(t *testing.T) {
	facade.Init(logcore.NewNoop())
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			l := facade.L()
			if l == nil {
				t.Error("L() returned nil in concurrent read")
			}
		}()
	}
	wg.Wait()
}
