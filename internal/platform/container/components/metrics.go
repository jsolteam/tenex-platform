package components

import (
	"context"
	"fmt"

	"github.com/jsolteam/tenex-platform/internal/platform/observability/metrics"
)

type MetricsComponent struct {
	cfg *ConfigComponent

	registry metrics.Registry
	shutdown func(context.Context) error
}

func NewMetrics(cfg *ConfigComponent) *MetricsComponent {
	return &MetricsComponent{cfg: cfg}
}

func (m *MetricsComponent) Start(ctx context.Context) error {
	port := m.cfg.Get().Observability.MetricsPort
	if port == 0 {
		port = 9090
	}

	reg := metrics.NewPrometheus()
	m.registry = reg
	m.shutdown = metrics.StartServer(ctx, port, reg.Handler())
	return nil
}

func (m *MetricsComponent) Stop(ctx context.Context) error {
	if m.shutdown != nil {
		if err := m.shutdown(ctx); err != nil {
			return fmt.Errorf("metrics server shutdown: %w", err)
		}
	}
	return nil
}

func (m *MetricsComponent) Registry() metrics.Registry {
	if m.registry == nil {
		panic("MetricsComponent.Registry() called before Start()")
	}
	return m.registry
}
