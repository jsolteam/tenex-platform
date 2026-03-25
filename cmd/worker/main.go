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
	s3Comp := components.NewS3(cfgComp, tracingComp, metricsComp)
	reposComp := components.NewRepositories(dbComp, tracingComp, metricsComp)
	workerSvcComp := components.NewWorkerService(reposComp, redisComp, s3Comp)

	c := container.New()
	c.Register("config", cfgComp)
	c.Register("metrics", metricsComp)
	c.Register("tracing", tracingComp)
	c.Register("logger", components.NewLogger(cfgComp))
	c.Register("db", dbComp)
	c.Register("redis", redisComp)
	c.Register("s3", s3Comp)
	c.Register("repositories", reposComp)
	c.Register("worker-service", workerSvcComp)

	if err := c.Start(ctx); err != nil {
		_, _ = os.Stderr.WriteString("worker: platform start failed: " + err.Error() + "\n")
		os.Exit(1)
	}

	l := facade.L()
	l.Info("worker: platform started")

	sm := shutdown.New(cfgComp.Get().App.ShutdownTimeout)

	sm.Register("updates", func(_ context.Context) error {
		l.Info("worker: stop accepting new tasks")
		return nil
	})

	sm.Register("workers", func(shutCtx context.Context) error {
		l.Info("worker: wait in-flight tasks")
		workerSvcComp.WaitInFlight(shutCtx)
		return nil
	})

	sm.Register("platform", func(shutCtx context.Context) error {
		l.Info("worker: flushing platform components")

		if err := c.Stop(shutCtx); err != nil {
			return apperrors.Wrap(err, apperrors.ErrInternal, "worker.shutdown.platform")
		}
		return nil
	})

	if err := sm.Wait(ctx); err != nil {
		l.Error("worker: shutdown error",
			zap.Error(err),
			zap.String("code", string(apperrors.CodeOf(err))),
		)
		os.Exit(1)
	}

	l.Info("worker: shutdown complete")
}
