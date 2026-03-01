package facade

import (
	"context"
	"sync/atomic"

	contextlog "github.com/jsol/tenex-platform/internal/platform/logger/context"
	"github.com/jsol/tenex-platform/internal/platform/logger/core"
)

var noop = core.NewNoop()

var global atomic.Pointer[core.Logger]

func Init(l *core.Logger) {
	global.Store(l)
}

func L() *core.Logger {
	if l := global.Load(); l != nil {
		return l
	}
	return noop
}

func Ctx(ctx context.Context) *core.Logger {
	return contextlog.FromCtx(ctx, L())
}
