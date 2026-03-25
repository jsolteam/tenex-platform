package components

import (
	"context"
	"runtime"
	"sync/atomic"

	"github.com/jsolteam/tenex-platform/internal/platform/logger/facade"
)

type SchedulerServiceComponent struct {
	repos *RepositoriesComponent
	redis *RedisComponent

	acceptGenerator atomic.Bool
	inCycle         atomic.Bool
}

func NewSchedulerService(repos *RepositoriesComponent, redis *RedisComponent) *SchedulerServiceComponent {
	return &SchedulerServiceComponent{repos: repos, redis: redis}
}

func (s *SchedulerServiceComponent) Start(_ context.Context) error {
	_ = s.repos.Repos()
	_ = s.redis.Client()
	s.acceptGenerator.Store(true)
	facade.L().Info("scheduler service started")
	return nil
}

func (s *SchedulerServiceComponent) Stop(_ context.Context) error {
	s.acceptGenerator.Store(false)
	facade.L().Info("scheduler service stopped")
	return nil
}

func (s *SchedulerServiceComponent) StopGenerator() {
	s.acceptGenerator.Store(false)
}

func (s *SchedulerServiceComponent) WaitCurrentCycle(_ context.Context) {
	for s.inCycle.Load() {
		runtime.Gosched()
	}
}
