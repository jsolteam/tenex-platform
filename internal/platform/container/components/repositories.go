package components

import (
	"context"
	"fmt"

	"github.com/jsolteam/tenex-platform/internal/infrastructure/database/repositories"
	"github.com/jsolteam/tenex-platform/internal/platform/logger/facade"
	"github.com/jsolteam/tenex-platform/internal/platform/observability/metrics"
	"github.com/jsolteam/tenex-platform/internal/platform/observability/tracing"
)

type RepositoriesComponent struct {
	db      *DBComponent
	tracing *TracingComponent
	metrics *MetricsComponent

	repos *repositories.Repositories
}

func NewRepositories(db *DBComponent, tracingComp *TracingComponent, metricsComp *MetricsComponent) *RepositoriesComponent {
	return &RepositoriesComponent{
		db:      db,
		tracing: tracingComp,
		metrics: metricsComp,
	}
}

func (r *RepositoriesComponent) Start(_ context.Context) error {
	log := facade.L()

	var tr tracing.Tracer
	if r.tracing != nil {
		tr = r.tracing.Tracer()
	} else {
		tr = tracing.NewNoop()
	}

	var (
		dbMet *metrics.DBMetrics
		err   error
	)
	if r.metrics != nil {
		dbMet, err = metrics.NewDBMetrics(r.metrics.Registry())
		if err != nil {
			return fmt.Errorf("repositories component metrics: %w", err)
		}
	} else {
		dbMet, _ = metrics.NewDBMetrics(metrics.NewNoop())
	}

	r.repos = repositories.New(r.db.SQL(), log, tr, dbMet)
	return nil
}

func (r *RepositoriesComponent) Stop(_ context.Context) error { return nil }

func (r *RepositoriesComponent) Repos() *repositories.Repositories {
	if r.repos == nil {
		panic("RepositoriesComponent.Repos() called before Start()")
	}
	return r.repos
}
