package middleware

import (
	"context"
	"time"

	"go.uber.org/zap"

	contextlog "github.com/jsol/tenex-platform/internal/platform/logger/context"
	"github.com/jsol/tenex-platform/internal/platform/logger/core"
)

type Handler func(ctx context.Context) error

func Observability(l *core.Logger) func(Handler) Handler {
	return func(next Handler) Handler {
		return func(ctx context.Context) error {
			log := contextlog.FromCtx(ctx, l)
			start := time.Now()

			log.Info("request.start")

			err := next(ctx)

			dur := time.Since(start)

			if err != nil {
				log.Error("request.failed",
					zap.Error(err),
					zap.Duration("duration", dur),
				)
			} else {
				log.Info("request.done",
					zap.Duration("duration", dur),
				)
			}

			return err
		}
	}
}

func PanicRecovery(l *core.Logger, fn func()) {
	defer func() {
		if r := recover(); r != nil {
			l.Error("panic.recovered", zap.Any("panic", r))
		}
	}()
	fn()
}
