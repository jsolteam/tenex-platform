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
	reposComp := components.NewRepositories(dbComp, tracingComp, metricsComp)
	schedulerSvcComp := components.NewSchedulerService(reposComp, redisComp)

	c := container.New()
	c.Register("config", cfgComp)
	c.Register("metrics", metricsComp)
	c.Register("tracing", tracingComp)
	c.Register("logger", components.NewLogger(cfgComp))
	c.Register("db", dbComp)
	c.Register("redis", redisComp)
	c.Register("repositories", reposComp)
	c.Register("scheduler-service", schedulerSvcComp)

	if err := c.Start(ctx); err != nil {
		_, _ = os.Stderr.WriteString("scheduler: platform start failed: " + err.Error() + "\n")
		os.Exit(1)
	}

	l := facade.L()
	l.Info("scheduler: platform started")

	sm := shutdown.New(cfgComp.Get().App.ShutdownTimeout)

	sm.Register("updates", func(_ context.Context) error {
		l.Info("scheduler: stop reminder generator")
		schedulerSvcComp.StopGenerator()
		return nil
	})

	sm.Register("workers", func(shutCtx context.Context) error {
		l.Info("scheduler: wait current cycle")
		schedulerSvcComp.WaitCurrentCycle(shutCtx)
		return nil
	})

	sm.Register("platform", func(shutCtx context.Context) error {
		l.Info("scheduler: flushing platform components")

		if err := c.Stop(shutCtx); err != nil {
			return apperrors.Wrap(err, apperrors.ErrInternal, "scheduler.shutdown.platform")
		}
		return nil
	})

	if err := sm.Wait(ctx); err != nil {
		l.Error("scheduler: shutdown error",
			zap.Error(err),
			zap.String("code", string(apperrors.CodeOf(err))),
		)
		os.Exit(1)
	}

	l.Info("scheduler: shutdown complete")
}
