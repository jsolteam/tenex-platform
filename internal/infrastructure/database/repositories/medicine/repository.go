package medicinerepo

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"go.uber.org/zap"

	"github.com/jsolteam/tenex-platform/internal/domain/medicine"
	"github.com/jsolteam/tenex-platform/internal/infrastructure/repolog"
	apperrors "github.com/jsolteam/tenex-platform/internal/platform/errors"
	"github.com/jsolteam/tenex-platform/internal/platform/logger/core"
	"github.com/jsolteam/tenex-platform/internal/platform/observability/metrics"
	"github.com/jsolteam/tenex-platform/internal/platform/observability/tracing"
)

const repoName = "medicine"

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

func (r *Repository) GetByID(ctx context.Context, id int64) (*medicine.Medicine, error) {
	const method = "GetByID"
	start := time.Now()
	ctx, span := r.tracer.Start(ctx, "medicinerepo."+method)
	defer span.End()
	defer func() { r.met.RecordDuration(ctx, repoName, method, time.Since(start).Seconds()) }()

	m := &medicine.Medicine{}
	err := r.db.QueryRowContext(ctx, `SELECT id, user_id, name, photo_media_id, created_at, updated_at FROM medicines WHERE id = $1`, id).
		Scan(&m.ID, &m.UserID, &m.Name, &m.PhotoMediaID, &m.CreatedAt, &m.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		repolog.Debug(ctx, r.log, "medicine not found", zap.Int64("id", id))
		return nil, medicine.ErrNotFound
	}
	if err != nil {
		appErr := apperrors.DB("medicinerepo."+method, err)
		repolog.Err(ctx, r.log, span, r.met, repoName, method, "failed to get medicine", appErr, zap.Int64("id", id))
		return nil, appErr
	}
	return m, nil
}

func (r *Repository) ListByUser(ctx context.Context, userID int64) ([]medicine.Medicine, error) {
	const method = "ListByUser"
	start := time.Now()
	ctx, span := r.tracer.Start(ctx, "medicinerepo."+method)
	defer span.End()
	defer func() { r.met.RecordDuration(ctx, repoName, method, time.Since(start).Seconds()) }()

	rows, err := r.db.QueryContext(ctx, `SELECT id, user_id, name, photo_media_id, created_at, updated_at FROM medicines WHERE user_id = $1 ORDER BY created_at ASC`, userID)
	if err != nil {
		appErr := apperrors.DB("medicinerepo."+method, err)
		repolog.Err(ctx, r.log, span, r.met, repoName, method, "failed to list medicines", appErr, zap.Int64("user_id", userID))
		return nil, appErr
	}
	defer rows.Close()

	var result []medicine.Medicine
	for rows.Next() {
		var m medicine.Medicine
		if err = rows.Scan(&m.ID, &m.UserID, &m.Name, &m.PhotoMediaID, &m.CreatedAt, &m.UpdatedAt); err != nil {
			appErr := apperrors.DB("medicinerepo."+method+".scan", err)
			repolog.Err(ctx, r.log, span, r.met, repoName, method, "failed to scan medicine", appErr, zap.Int64("user_id", userID))
			return nil, appErr
		}
		result = append(result, m)
	}
	if err = rows.Err(); err != nil {
		appErr := apperrors.DB("medicinerepo."+method+".rows", err)
		repolog.Err(ctx, r.log, span, r.met, repoName, method, "medicine rows error", appErr, zap.Int64("user_id", userID))
		return nil, appErr
	}
	return result, nil
}

func (r *Repository) Create(ctx context.Context, m *medicine.Medicine) error {
	const method = "Create"
	start := time.Now()
	ctx, span := r.tracer.Start(ctx, "medicinerepo."+method)
	defer span.End()
	defer func() { r.met.RecordDuration(ctx, repoName, method, time.Since(start).Seconds()) }()

	err := r.db.QueryRowContext(ctx, `INSERT INTO medicines (user_id, name, photo_media_id) VALUES ($1, $2, $3) RETURNING id, created_at, updated_at`, m.UserID, m.Name, m.PhotoMediaID).
		Scan(&m.ID, &m.CreatedAt, &m.UpdatedAt)
	if err != nil {
		appErr := apperrors.DB("medicinerepo."+method, err)
		repolog.Err(ctx, r.log, span, r.met, repoName, method, "failed to create medicine", appErr,
			zap.Int64("user_id", m.UserID),
			zap.String("name", m.Name),
		)
		return appErr
	}
	return nil
}

func (r *Repository) Update(ctx context.Context, m *medicine.Medicine) error {
	const method = "Update"
	start := time.Now()
	ctx, span := r.tracer.Start(ctx, "medicinerepo."+method)
	defer span.End()
	defer func() { r.met.RecordDuration(ctx, repoName, method, time.Since(start).Seconds()) }()

	err := r.db.QueryRowContext(ctx, `UPDATE medicines SET name = $1, photo_media_id = $2, updated_at = now() WHERE id = $3 RETURNING updated_at`, m.Name, m.PhotoMediaID, m.ID).
		Scan(&m.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		repolog.Debug(ctx, r.log, "medicine not found on update", zap.Int64("id", m.ID))
		return medicine.ErrNotFound
	}
	if err != nil {
		appErr := apperrors.DB("medicinerepo."+method, err)
		repolog.Err(ctx, r.log, span, r.met, repoName, method, "failed to update medicine", appErr, zap.Int64("id", m.ID))
		return appErr
	}
	return nil
}

func (r *Repository) Delete(ctx context.Context, id int64) error {
	const method = "Delete"
	start := time.Now()
	ctx, span := r.tracer.Start(ctx, "medicinerepo."+method)
	defer span.End()
	defer func() { r.met.RecordDuration(ctx, repoName, method, time.Since(start).Seconds()) }()

	res, err := r.db.ExecContext(ctx, `DELETE FROM medicines WHERE id = $1`, id)
	if err != nil {
		appErr := apperrors.DB("medicinerepo."+method, err)
		repolog.Err(ctx, r.log, span, r.met, repoName, method, "failed to delete medicine", appErr, zap.Int64("id", id))
		return appErr
	}
	if n, _ := res.RowsAffected(); n == 0 {
		repolog.Debug(ctx, r.log, "medicine not found on delete", zap.Int64("id", id))
		return medicine.ErrNotFound
	}
	return nil
}
