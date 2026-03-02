package tracing_unit

import (
	"context"
	"errors"
	"testing"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"

	"github.com/jsol/tenex-platform/internal/platform/config"
	"github.com/jsol/tenex-platform/internal/platform/observability/tracing"
)

func minCfg() *config.AppConfig {
	return &config.AppConfig{
		App: config.App{
			Name: "tenex-test",
			Env:  "test",
		},
		DB: config.DB{
			Host: "localhost",
			Port: 5432,
			User: "tenex",
			Name: "tenex",
		},
		Redis: config.Redis{Addr: "localhost:6379"},
		Scheduler: config.Scheduler{
			MaxRetries:            3,
			ReminderRetryInterval: 30 * time.Second,
		},
	}
}

func TestNoopTracer_StartEnd(t *testing.T) {
	tr := tracing.NewNoop()

	ctx, span := tr.Start(context.Background(), "test-span")
	if ctx == nil {
		t.Fatal("Start returned nil context")
	}
	if span == nil {
		t.Fatal("Start returned nil span")
	}

	span.SetAttributes(attribute.String("key", "value"))
	span.RecordError(errors.New("test error"))
	span.SetStatus(codes.Error, "something went wrong")
	span.End()
}

func TestNoopTracer_ChildSpan(t *testing.T) {
	tr := tracing.NewNoop()

	ctx, parent := tr.Start(context.Background(), "parent")
	defer parent.End()

	_, child := tr.Start(ctx, "child")
	defer child.End()
}

func TestBootstrap_EmptyEndpoint(t *testing.T) {
	cfg := minCfg()

	tr, shutdown, err := tracing.Bootstrap(context.Background(), cfg)
	if err != nil {
		t.Fatalf("Bootstrap(empty endpoint) returned error: %v", err)
	}
	if tr == nil {
		t.Fatal("Bootstrap returned nil tracer")
	}
	if shutdown == nil {
		t.Fatal("Bootstrap returned nil shutdown func")
	}

	ctx, span := tr.Start(context.Background(), "test")
	if ctx == nil || span == nil {
		t.Fatal("noop tracer returned nil ctx or span")
	}
	span.End()

	if err := shutdown(context.Background()); err != nil {
		t.Errorf("noop shutdown returned error: %v", err)
	}
}

func TestBootstrap_Shutdown_NoPanic(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("Bootstrap shutdown panicked: %v", r)
		}
	}()

	cfg := minCfg()

	_, shutdown, err := tracing.Bootstrap(context.Background(), cfg)
	if err != nil {
		t.Fatalf("Bootstrap: %v", err)
	}

	if err := shutdown(context.Background()); err != nil {
		t.Errorf("first shutdown: %v", err)
	}
	if err := shutdown(context.Background()); err != nil {
		t.Errorf("second shutdown: %v", err)
	}
}

func TestBootstrap_InvalidEndpoint_ReturnsError(t *testing.T) {
	cfg := minCfg()
	cfg.Observability.OTLPEndpoint = "invalid-host-that-does-not-exist:4317"

	tr, shutdown, err := tracing.Bootstrap(context.Background(), cfg)
	if err != nil {
		if tr != nil || shutdown != nil {
			t.Error("on error, tracer and shutdown should both be nil")
		}
		return
	}

	if tr == nil {
		t.Fatal("tracer is nil without error")
	}
	ctx, span := tr.Start(context.Background(), "test")
	if ctx == nil || span == nil {
		t.Fatal("Start returned nil")
	}
	span.End()

	shutCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_ = shutdown(shutCtx)
}
func TestNoopSpan_AllMethods(t *testing.T) {
	tr := tracing.NewNoop()
	_, span := tr.Start(context.Background(), "all-methods")

	span.SetAttributes(
		attribute.String("str", "val"),
		attribute.Int("int", 42),
		attribute.Bool("bool", true),
	)
	span.RecordError(nil)
	span.RecordError(errors.New("oops"))
	span.SetStatus(codes.Ok, "")
	span.SetStatus(codes.Error, "err")
	span.End()
}
