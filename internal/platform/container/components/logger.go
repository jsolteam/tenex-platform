package components

import (
	"context"
	"fmt"

	"github.com/jsol/tenex-platform/internal/platform/logger"
)

type LoggerComponent struct {
	cfg *ConfigComponent

	shutdown func() error
}

func NewLogger(cfg *ConfigComponent) *LoggerComponent {
	return &LoggerComponent{cfg: cfg}
}

func (l *LoggerComponent) Start(_ context.Context) error {
	shutdown, err := logger.Bootstrap(l.cfg.Manager())
	if err != nil {
		return fmt.Errorf("logger bootstrap: %w", err)
	}
	l.shutdown = shutdown
	return nil
}

func (l *LoggerComponent) Stop(_ context.Context) error {
	if l.shutdown != nil {
		if err := l.shutdown(); err != nil {
			return fmt.Errorf("logger shutdown: %w", err)
		}
	}
	return nil
}
