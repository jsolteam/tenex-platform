package configrepo

import (
	"context"
	"database/sql"
	"errors"

	"go.uber.org/zap"

	domainconfig "github.com/jsolteam/tenex-platform/internal/domain/config"
	"github.com/jsolteam/tenex-platform/internal/infrastructure/repolog"
	apperrors "github.com/jsolteam/tenex-platform/internal/platform/errors"
	"github.com/jsolteam/tenex-platform/internal/platform/logger/core"
	"github.com/jsolteam/tenex-platform/internal/platform/observability/tracing"
)

type Repository struct {
	db     *sql.DB
	log    *core.Logger
	tracer tracing.Tracer
}

func New(db *sql.DB, log *core.Logger, tracer tracing.Tracer) *Repository {
	return &Repository{
		db:     db,
		log:    log.With(zap.String("repo", "config")),
		tracer: tracer,
	}
}

func (r *Repository) Get(ctx context.Context, key string) (*domainconfig.PlatformConfig, error) {
	ctx, span := r.tracer.Start(ctx, "configrepo.Get")
	defer span.End()

	c := &domainconfig.PlatformConfig{}
	err := r.db.QueryRowContext(ctx, `SELECT key, value, updated_at FROM platform_config WHERE key=$1`, key).
		Scan(&c.Key, &c.Value, &c.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		repolog.Debug(ctx, r.log, "config key not found", zap.String("key", key))
		return nil, domainconfig.ErrNotFound
	}
	if err != nil {
		appErr := apperrors.DB("configrepo.Get", err)
		repolog.Err(ctx, r.log, span, "failed to get config", appErr, zap.String("key", key))
		return nil, appErr
	}
	return c, nil
}

func (r *Repository) GetAll(ctx context.Context) ([]domainconfig.PlatformConfig, error) {
	ctx, span := r.tracer.Start(ctx, "configrepo.GetAll")
	defer span.End()

	rows, err := r.db.QueryContext(ctx, `SELECT key, value, updated_at FROM platform_config ORDER BY key ASC`)
	if err != nil {
		appErr := apperrors.DB("configrepo.GetAll", err)
		repolog.Err(ctx, r.log, span, "failed to get all config", appErr)
		return nil, appErr
	}
	defer rows.Close()

	var result []domainconfig.PlatformConfig
	for rows.Next() {
		var c domainconfig.PlatformConfig
		if err = rows.Scan(&c.Key, &c.Value, &c.UpdatedAt); err != nil {
			appErr := apperrors.DB("configrepo.GetAll.scan", err)
			repolog.Err(ctx, r.log, span, "failed to scan config row", appErr)
			return nil, appErr
		}
		result = append(result, c)
	}
	if err = rows.Err(); err != nil {
		appErr := apperrors.DB("configrepo.GetAll.rows", err)
		repolog.Err(ctx, r.log, span, "config rows error", appErr)
		return nil, appErr
	}
	return result, nil
}

func (r *Repository) Set(ctx context.Context, key, value string) error {
	ctx, span := r.tracer.Start(ctx, "configrepo.Set")
	defer span.End()

	const q = `
		INSERT INTO platform_config (key, value, updated_at) VALUES ($1,$2,now())
		ON CONFLICT (key) DO UPDATE SET value=EXCLUDED.value, updated_at=now()`

	_, err := r.db.ExecContext(ctx, q, key, value)
	if err != nil {
		appErr := apperrors.DB("configrepo.Set", err)
		repolog.Err(ctx, r.log, span, "failed to set config", appErr, zap.String("key", key))
		return appErr
	}
	return nil
}

func (r *Repository) Delete(ctx context.Context, key string) error {
	ctx, span := r.tracer.Start(ctx, "configrepo.Delete")
	defer span.End()

	res, err := r.db.ExecContext(ctx, `DELETE FROM platform_config WHERE key=$1`, key)
	if err != nil {
		appErr := apperrors.DB("configrepo.Delete", err)
		repolog.Err(ctx, r.log, span, "failed to delete config", appErr, zap.String("key", key))
		return appErr
	}
	if n, _ := res.RowsAffected(); n == 0 {
		repolog.Debug(ctx, r.log, "config key not found on delete", zap.String("key", key))
		return domainconfig.ErrNotFound
	}
	return nil
}
