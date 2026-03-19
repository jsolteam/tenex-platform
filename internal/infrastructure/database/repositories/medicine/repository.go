package medicinerepo

import (
	"context"
	"database/sql"
	"errors"

	"go.uber.org/zap"

	"github.com/jsolteam/tenex-platform/internal/domain/medicine"
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
		log:    log.With(zap.String("repo", "medicine")),
		tracer: tracer,
	}
}

func (r *Repository) GetByID(ctx context.Context, id int64) (*medicine.Medicine, error) {
	ctx, span := r.tracer.Start(ctx, "medicinerepo.GetByID")
	defer span.End()

	const q = `SELECT id, user_id, name, photo_media_id, created_at, updated_at FROM medicines WHERE id = $1`

	m := &medicine.Medicine{}
	err := r.db.QueryRowContext(ctx, q, id).Scan(
		&m.ID, &m.UserID, &m.Name, &m.PhotoMediaID, &m.CreatedAt, &m.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		repolog.Debug(ctx, r.log, "medicine not found", zap.Int64("id", id))
		return nil, medicine.ErrNotFound
	}
	if err != nil {
		appErr := apperrors.DB("medicinerepo.GetByID", err)
		repolog.Err(ctx, r.log, span, "failed to get medicine", appErr, zap.Int64("id", id))
		return nil, appErr
	}
	return m, nil
}

func (r *Repository) ListByUser(ctx context.Context, userID int64) ([]medicine.Medicine, error) {
	ctx, span := r.tracer.Start(ctx, "medicinerepo.ListByUser")
	defer span.End()

	const q = `SELECT id, user_id, name, photo_media_id, created_at, updated_at FROM medicines WHERE user_id = $1 ORDER BY created_at ASC`

	rows, err := r.db.QueryContext(ctx, q, userID)
	if err != nil {
		appErr := apperrors.DB("medicinerepo.ListByUser", err)
		repolog.Err(ctx, r.log, span, "failed to list medicines", appErr, zap.Int64("user_id", userID))
		return nil, appErr
	}
	defer rows.Close()

	var result []medicine.Medicine
	for rows.Next() {
		var m medicine.Medicine
		if err = rows.Scan(&m.ID, &m.UserID, &m.Name, &m.PhotoMediaID, &m.CreatedAt, &m.UpdatedAt); err != nil {
			appErr := apperrors.DB("medicinerepo.ListByUser.scan", err)
			repolog.Err(ctx, r.log, span, "failed to scan medicine", appErr, zap.Int64("user_id", userID))
			return nil, appErr
		}
		result = append(result, m)
	}
	if err = rows.Err(); err != nil {
		appErr := apperrors.DB("medicinerepo.ListByUser.rows", err)
		repolog.Err(ctx, r.log, span, "medicine rows error", appErr, zap.Int64("user_id", userID))
		return nil, appErr
	}
	return result, nil
}

func (r *Repository) Create(ctx context.Context, m *medicine.Medicine) error {
	ctx, span := r.tracer.Start(ctx, "medicinerepo.Create")
	defer span.End()

	const q = `INSERT INTO medicines (user_id, name, photo_media_id) VALUES ($1, $2, $3) RETURNING id, created_at, updated_at`

	err := r.db.QueryRowContext(ctx, q, m.UserID, m.Name, m.PhotoMediaID).
		Scan(&m.ID, &m.CreatedAt, &m.UpdatedAt)
	if err != nil {
		appErr := apperrors.DB("medicinerepo.Create", err)
		repolog.Err(ctx, r.log, span, "failed to create medicine", appErr,
			zap.Int64("user_id", m.UserID),
			zap.String("name", m.Name),
		)
		return appErr
	}
	return nil
}

func (r *Repository) Update(ctx context.Context, m *medicine.Medicine) error {
	ctx, span := r.tracer.Start(ctx, "medicinerepo.Update")
	defer span.End()

	const q = `UPDATE medicines SET name = $1, photo_media_id = $2, updated_at = now() WHERE id = $3 RETURNING updated_at`

	err := r.db.QueryRowContext(ctx, q, m.Name, m.PhotoMediaID, m.ID).Scan(&m.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		repolog.Debug(ctx, r.log, "medicine not found on update", zap.Int64("id", m.ID))
		return medicine.ErrNotFound
	}
	if err != nil {
		appErr := apperrors.DB("medicinerepo.Update", err)
		repolog.Err(ctx, r.log, span, "failed to update medicine", appErr, zap.Int64("id", m.ID))
		return appErr
	}
	return nil
}

func (r *Repository) Delete(ctx context.Context, id int64) error {
	ctx, span := r.tracer.Start(ctx, "medicinerepo.Delete")
	defer span.End()

	res, err := r.db.ExecContext(ctx, `DELETE FROM medicines WHERE id = $1`, id)
	if err != nil {
		appErr := apperrors.DB("medicinerepo.Delete", err)
		repolog.Err(ctx, r.log, span, "failed to delete medicine", appErr, zap.Int64("id", id))
		return appErr
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		repolog.Debug(ctx, r.log, "medicine not found on delete", zap.Int64("id", id))
		return medicine.ErrNotFound
	}
	return nil
}
