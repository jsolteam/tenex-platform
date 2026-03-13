package contextlog

import (
	"context"

	"github.com/jsolteam/tenex-platform/internal/platform/logger/core"
	"github.com/jsolteam/tenex-platform/internal/platform/logger/tracing"
)

func FromCtx(ctx context.Context, l *core.Logger) *core.Logger {
	fields := Extract(ctx)
	fields = append(fields, tracing.Inject(ctx)...)
	if len(fields) == 0 {
		return l
	}
	return l.With(fields...)
}
