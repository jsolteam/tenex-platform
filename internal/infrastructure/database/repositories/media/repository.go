package mediarepo

import (
	"context"
	"database/sql"
	"errors"

	"go.uber.org/zap"

	"github.com/jsolteam/tenex-platform/internal/domain/media"
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
		log:    log.With(zap.String("repo", "media")),
		tracer: tracer,
	}
}

func (r *Repository) GetByID(ctx context.Context, id int64) (*media.Media, error) {
	ctx, span := r.tracer.Start(ctx, "mediarepo.GetByID")
	defer span.End()

	m := &media.Media{}
	err := r.db.QueryRowContext(ctx, `SELECT id, owner_user_id, media_type, created_at FROM media WHERE id=$1`, id).
		Scan(&m.ID, &m.OwnerUserID, &m.MediaType, &m.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		repolog.Debug(ctx, r.log, "media not found", zap.Int64("id", id))
		return nil, media.ErrNotFound
	}
	if err != nil {
		appErr := apperrors.DB("mediarepo.GetByID", err)
		repolog.Err(ctx, r.log, span, "failed to get media", appErr, zap.Int64("id", id))
		return nil, appErr
	}
	return m, nil
}

func (r *Repository) Create(ctx context.Context, m *media.Media) error {
	ctx, span := r.tracer.Start(ctx, "mediarepo.Create")
	defer span.End()

	err := r.db.QueryRowContext(ctx, `INSERT INTO media (owner_user_id, media_type) VALUES ($1,$2) RETURNING id, created_at`, m.OwnerUserID, m.MediaType).
		Scan(&m.ID, &m.CreatedAt)
	if err != nil {
		appErr := apperrors.DB("mediarepo.Create", err)
		repolog.Err(ctx, r.log, span, "failed to create media", appErr,
			zap.Int64("owner_user_id", m.OwnerUserID),
			zap.String("media_type", string(m.MediaType)),
		)
		return appErr
	}
	return nil
}

func (r *Repository) Delete(ctx context.Context, id int64) error {
	ctx, span := r.tracer.Start(ctx, "mediarepo.Delete")
	defer span.End()

	res, err := r.db.ExecContext(ctx, `DELETE FROM media WHERE id=$1`, id)
	if err != nil {
		appErr := apperrors.DB("mediarepo.Delete", err)
		repolog.Err(ctx, r.log, span, "failed to delete media", appErr, zap.Int64("id", id))
		return appErr
	}
	if n, _ := res.RowsAffected(); n == 0 {
		repolog.Debug(ctx, r.log, "media not found on delete", zap.Int64("id", id))
		return media.ErrNotFound
	}
	return nil
}

func (r *Repository) GetVariants(ctx context.Context, mediaID int64) ([]media.MediaVariant, error) {
	ctx, span := r.tracer.Start(ctx, "mediarepo.GetVariants")
	defer span.End()

	const q = `SELECT id, media_id, storage_type, COALESCE(messenger,''), COALESCE(external_id,''), COALESCE(url,''), created_at FROM media_variants WHERE media_id=$1 ORDER BY created_at ASC`

	rows, err := r.db.QueryContext(ctx, q, mediaID)
	if err != nil {
		appErr := apperrors.DB("mediarepo.GetVariants", err)
		repolog.Err(ctx, r.log, span, "failed to get media variants", appErr, zap.Int64("media_id", mediaID))
		return nil, appErr
	}
	defer rows.Close()

	var variants []media.MediaVariant
	for rows.Next() {
		var v media.MediaVariant
		if err = rows.Scan(&v.ID, &v.MediaID, &v.StorageType, &v.Messenger, &v.ExternalID, &v.URL, &v.CreatedAt); err != nil {
			appErr := apperrors.DB("mediarepo.GetVariants.scan", err)
			repolog.Err(ctx, r.log, span, "failed to scan media variant", appErr, zap.Int64("media_id", mediaID))
			return nil, appErr
		}
		variants = append(variants, v)
	}
	if err = rows.Err(); err != nil {
		appErr := apperrors.DB("mediarepo.GetVariants.rows", err)
		repolog.Err(ctx, r.log, span, "media variants rows error", appErr, zap.Int64("media_id", mediaID))
		return nil, appErr
	}
	return variants, nil
}

func (r *Repository) GetVariantByStorage(ctx context.Context, mediaID int64, storageType media.StorageType, messenger string) (*media.MediaVariant, error) {
	ctx, span := r.tracer.Start(ctx, "mediarepo.GetVariantByStorage")
	defer span.End()

	const q = `SELECT id, media_id, storage_type, COALESCE(messenger,''), COALESCE(external_id,''), COALESCE(url,''), created_at FROM media_variants WHERE media_id=$1 AND storage_type=$2 AND COALESCE(messenger,'')=$3`

	v := &media.MediaVariant{}
	err := r.db.QueryRowContext(ctx, q, mediaID, storageType, messenger).
		Scan(&v.ID, &v.MediaID, &v.StorageType, &v.Messenger, &v.ExternalID, &v.URL, &v.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		repolog.Debug(ctx, r.log, "media variant not found",
			zap.Int64("media_id", mediaID),
			zap.String("storage_type", string(storageType)),
		)
		return nil, media.ErrVariantNotFound
	}
	if err != nil {
		appErr := apperrors.DB("mediarepo.GetVariantByStorage", err)
		repolog.Err(ctx, r.log, span, "failed to get media variant", appErr, zap.Int64("media_id", mediaID))
		return nil, appErr
	}
	return v, nil
}

func (r *Repository) AddVariant(ctx context.Context, v *media.MediaVariant) error {
	ctx, span := r.tracer.Start(ctx, "mediarepo.AddVariant")
	defer span.End()

	const q = `INSERT INTO media_variants (media_id, storage_type, messenger, external_id, url) VALUES ($1,$2,NULLIF($3,''),NULLIF($4,''),NULLIF($5,'')) RETURNING id, created_at`

	err := r.db.QueryRowContext(ctx, q, v.MediaID, v.StorageType, v.Messenger, v.ExternalID, v.URL).Scan(&v.ID, &v.CreatedAt)
	if err != nil {
		appErr := apperrors.DB("mediarepo.AddVariant", err)
		repolog.Err(ctx, r.log, span, "failed to add media variant", appErr,
			zap.Int64("media_id", v.MediaID),
			zap.String("storage_type", string(v.StorageType)),
		)
		return appErr
	}
	return nil
}

func (r *Repository) DeleteVariant(ctx context.Context, id int64) error {
	ctx, span := r.tracer.Start(ctx, "mediarepo.DeleteVariant")
	defer span.End()

	res, err := r.db.ExecContext(ctx, `DELETE FROM media_variants WHERE id=$1`, id)
	if err != nil {
		appErr := apperrors.DB("mediarepo.DeleteVariant", err)
		repolog.Err(ctx, r.log, span, "failed to delete media variant", appErr, zap.Int64("id", id))
		return appErr
	}
	if n, _ := res.RowsAffected(); n == 0 {
		repolog.Debug(ctx, r.log, "media variant not found on delete", zap.Int64("id", id))
		return media.ErrVariantNotFound
	}
	return nil
}
