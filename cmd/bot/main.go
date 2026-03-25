package main

import (
	"context"
	"os"

	"github.com/jsolteam/tenex-platform/internal/platform/container"
	"github.com/jsolteam/tenex-platform/internal/platform/container/components"
	apperrors "github.com/jsolteam/tenex-platform/internal/platform/errors"
	"github.com/jsolteam/tenex-platform/internal/platform/logger/facade"
	"github.com/jsolteam/tenex-platform/internal/platform/shutdown"
	"go.uber.org/zap"
)

func main() {
	ctx := context.Background()

	cfgComp := components.NewConfig(os.Getenv("CONFIG_FILE"))
	metricsComp := components.NewMetrics(cfgComp)
	tracingComp := components.NewTracing(cfgComp)
	dbComp := components.NewDB(cfgComp)
	redisComp := components.NewRedis(cfgComp, tracingComp, metricsComp)
	s3Comp := components.NewS3(cfgComp, tracingComp)
	reposComp := components.NewRepositories(dbComp, tracingComp, metricsComp)
	botComp := components.NewBot(cfgComp, reposComp, os.Getenv("MESSENGER"), os.Getenv("MESSENGER_TOKEN"))

	c := container.New()
	c.Register("config", cfgComp)
	c.Register("metrics", metricsComp)
	c.Register("tracing", tracingComp)
	c.Register("logger", components.NewLogger(cfgComp))
	c.Register("db", dbComp)
	c.Register("redis", redisComp)
	c.Register("s3", s3Comp)
	c.Register("repositories", reposComp)
	c.Register("bot", botComp)

	if err := c.Start(ctx); err != nil {
		_, _ = os.Stderr.WriteString("bot: platform start failed: " + err.Error() + "\n")
		os.Exit(1)
	}

	l := facade.L()
	l.Info("bot: platform started")

	sm := shutdown.New(cfgComp.Get().App.ShutdownTimeout)

	sm.Register("updates", func(_ context.Context) error {
		l.Info("bot: stop accepting updates")
		return nil
	})

	sm.Register("workers", func(shutCtx context.Context) error {
		l.Info("bot: wait in-flight workers")
		botComp.WaitWorkers(shutCtx)
		return nil
	})

	sm.Register("platform", func(shutCtx context.Context) error {
		l.Info("bot: flushing platform components")

		if err := c.Stop(shutCtx); err != nil {
			return apperrors.Wrap(err, apperrors.ErrInternal, "bot.shutdown.platform")
		}
		return nil
	})

	if err := sm.Wait(ctx); err != nil {
		l.Error("bot: shutdown error",
			zap.Error(err),
			zap.String("code", string(apperrors.CodeOf(err))),
		)
		os.Exit(1)
	}

	l.Info("bot: shutdown complete")
}
