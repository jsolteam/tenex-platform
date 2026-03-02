package components

import (
	"context"
	"fmt"

	"github.com/jsol/tenex-platform/internal/platform/observability/tracing"
)

type TracingComponent struct {
	cfg *ConfigComponent

	tracer   tracing.Tracer
	shutdown func(context.Context) error
}

func NewTracing(cfg *ConfigComponent) *TracingComponent {
	return &TracingComponent{cfg: cfg}
}

// Start calls tracing.Bootstrap with the current AppConfig.
func (t *TracingComponent) Start(ctx context.Context) error {
	tr, shutdown, err := tracing.Bootstrap(ctx, t.cfg.Get())
	if err != nil {
		return fmt.Errorf("tracing bootstrap: %w", err)
	}
	t.tracer = tr
	t.shutdown = shutdown
	return nil
}

func (t *TracingComponent) Stop(ctx context.Context) error {
	if t.shutdown != nil {
		if err := t.shutdown(ctx); err != nil {
			return fmt.Errorf("tracing shutdown: %w", err)
		}
	}
	return nil
}

func (t *TracingComponent) Tracer() tracing.Tracer {
	if t.tracer == nil {
		panic("TracingComponent.Tracer() called before Start()")
	}
	return t.tracer
}
