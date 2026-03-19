package configrepo

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"go.uber.org/zap"

	domainconfig "github.com/jsolteam/tenex-platform/internal/domain/config"
	"github.com/jsolteam/tenex-platform/internal/infrastructure/repolog"
	apperrors "github.com/jsolteam/tenex-platform/internal/platform/errors"
	"github.com/jsolteam/tenex-platform/internal/platform/logger/core"
	"github.com/jsolteam/tenex-platform/internal/platform/observability/metrics"
	"github.com/jsolteam/tenex-platform/internal/platform/observability/tracing"
)

const repoName = "config"

type Repository struct {
	db     *sql.DB
	log    *core.Logger
	tracer tracing.Tracer
	met    *metrics.DBMetrics
}

func New(db *sql.DB, log *core.Logger, tracer tracing.Tracer, met *metrics.DBMetrics) *Repository {
	return &Repository{
		db:     db,
		log:    log.With(zap.String("repo", repoName)),
		tracer: tracer,
		met:    met,
	}
}

func (r *Repository) Get(ctx context.Context, key string) (*domainconfig.PlatformConfig, error) {
	const method = "Get"
	start := time.Now()
	ctx, span := r.tracer.Start(ctx, "configrepo."+method)
	defer span.End()
	defer func() { r.met.RecordDuration(ctx, repoName, method, time.Since(start).Seconds()) }()

	c := &domainconfig.PlatformConfig{}
	err := r.db.QueryRowContext(ctx, `SELECT key, value, updated_at FROM platform_config WHERE key=$1`, key).
		Scan(&c.Key, &c.Value, &c.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		repolog.Debug(ctx, r.log, "config key not found", zap.String("key", key))
		return nil, domainconfig.ErrNotFound
	}
	if err != nil {
		appErr := apperrors.DB("configrepo."+method, err)
		repolog.Err(ctx, r.log, span, r.met, repoName, method, "failed to get config", appErr, zap.String("key", key))
		return nil, appErr
	}
	return c, nil
}

func (r *Repository) GetAll(ctx context.Context) ([]domainconfig.PlatformConfig, error) {
	const method = "GetAll"
	start := time.Now()
	ctx, span := r.tracer.Start(ctx, "configrepo."+method)
	defer span.End()
	defer func() { r.met.RecordDuration(ctx, repoName, method, time.Since(start).Seconds()) }()

	rows, err := r.db.QueryContext(ctx, `SELECT key, value, updated_at FROM platform_config ORDER BY key ASC`)
	if err != nil {
		appErr := apperrors.DB("configrepo."+method, err)
		repolog.Err(ctx, r.log, span, r.met, repoName, method, "failed to get all config", appErr)
		return nil, appErr
	}
	defer rows.Close()

	var result []domainconfig.PlatformConfig
	for rows.Next() {
		var c domainconfig.PlatformConfig
		if err = rows.Scan(&c.Key, &c.Value, &c.UpdatedAt); err != nil {
			appErr := apperrors.DB("configrepo."+method+".scan", err)
			repolog.Err(ctx, r.log, span, r.met, repoName, method, "failed to scan config row", appErr)
			return nil, appErr
		}
		result = append(result, c)
	}
	if err = rows.Err(); err != nil {
		appErr := apperrors.DB("configrepo."+method+".rows", err)
		repolog.Err(ctx, r.log, span, r.met, repoName, method, "config rows error", appErr)
		return nil, appErr
	}
	return result, nil
}

func (r *Repository) Set(ctx context.Context, key, value string) error {
	const method = "Set"
	start := time.Now()
	ctx, span := r.tracer.Start(ctx, "configrepo."+method)
	defer span.End()
	defer func() { r.met.RecordDuration(ctx, repoName, method, time.Since(start).Seconds()) }()

	_, err := r.db.ExecContext(ctx, `INSERT INTO platform_config (key, value, updated_at) VALUES ($1,$2,now()) ON CONFLICT (key) DO UPDATE SET value=EXCLUDED.value, updated_at=now()`, key, value)
	if err != nil {
		appErr := apperrors.DB("configrepo."+method, err)
		repolog.Err(ctx, r.log, span, r.met, repoName, method, "failed to set config", appErr, zap.String("key", key))
		return appErr
	}
	return nil
}

func (r *Repository) Delete(ctx context.Context, key string) error {
	const method = "Delete"
	start := time.Now()
	ctx, span := r.tracer.Start(ctx, "configrepo."+method)
	defer span.End()
	defer func() { r.met.RecordDuration(ctx, repoName, method, time.Since(start).Seconds()) }()

	res, err := r.db.ExecContext(ctx, `DELETE FROM platform_config WHERE key=$1`, key)
	if err != nil {
		appErr := apperrors.DB("configrepo."+method, err)
		repolog.Err(ctx, r.log, span, r.met, repoName, method, "failed to delete config", appErr, zap.String("key", key))
		return appErr
	}
	if n, _ := res.RowsAffected(); n == 0 {
		repolog.Debug(ctx, r.log, "config key not found on delete", zap.String("key", key))
		return domainconfig.ErrNotFound
	}
	return nil
}
