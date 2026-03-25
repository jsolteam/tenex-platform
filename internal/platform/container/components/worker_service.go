package components

import (
	"context"
	"runtime"
	"sync/atomic"

	"github.com/jsolteam/tenex-platform/internal/platform/logger/facade"
)

type WorkerServiceComponent struct {
	repos *RepositoriesComponent
	redis *RedisComponent
	s3    *S3Component

	inflight atomic.Int64
}

func NewWorkerService(repos *RepositoriesComponent, redis *RedisComponent, s3 *S3Component) *WorkerServiceComponent {
	return &WorkerServiceComponent{repos: repos, redis: redis, s3: s3}
}

func (w *WorkerServiceComponent) Start(_ context.Context) error {
	_ = w.repos.Repos()
	_ = w.redis.Client()
	_ = w.s3.Client()
	facade.L().Info("worker service started")
	return nil
}

func (w *WorkerServiceComponent) Stop(_ context.Context) error {
	facade.L().Info("worker service stopped")
	return nil
}

func (w *WorkerServiceComponent) WaitInFlight(_ context.Context) {
	for w.inflight.Load() > 0 {
		runtime.Gosched()
	}
}
