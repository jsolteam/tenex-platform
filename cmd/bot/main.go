package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/jsol/tenex-platform/internal/platform/container"
	"github.com/jsol/tenex-platform/internal/platform/container/components"
	"github.com/jsol/tenex-platform/internal/platform/logger/facade"
	"go.uber.org/zap"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer stop()

	cfgComp := components.NewConfig(os.Getenv("CONFIG_FILE"))

	c := container.New()
	c.Register("config", cfgComp)
	c.Register("metrics", components.NewMetrics(cfgComp))
	c.Register("tracing", components.NewTracing(cfgComp))
	c.Register("logger", components.NewLogger(cfgComp))

	if err := c.Start(ctx); err != nil {
		_, _ = os.Stderr.WriteString("bot: container start failed: " + err.Error() + "\n")
		os.Exit(1)
	}

	facade.L().Info("bot: started")

	<-ctx.Done()

	facade.L().Info("bot: shutting down")

	shutdownTimeout := cfgComp.Get().App.ShutdownTimeout
	shutCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()

	if err := c.Stop(shutCtx); err != nil {
		facade.L().Error("bot: shutdown error", zap.Error(err))
		os.Exit(1)
	}

	facade.L().Info("bot: shutdown complete")
}
