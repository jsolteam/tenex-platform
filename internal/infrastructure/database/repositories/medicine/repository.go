package medicinerepo

import (
	"context"
	"database/sql"
	"errors"

	"github.com/jsolteam/tenex-platform/internal/domain/medicine"
	apperrors "github.com/jsolteam/tenex-platform/internal/platform/errors"
)

type Repository struct {
	db *sql.DB
}

func New(db *sql.DB) *Repository {
	return &Repository{db: db}
}

// GetByID возвращает лекарство по ID.
func (r *Repository) GetByID(ctx context.Context, id int64) (*medicine.Medicine, error) {
	const q = `
		SELECT id, user_id, name, photo_media_id, created_at, updated_at
		FROM medicines
		WHERE id = $1`

	m := &medicine.Medicine{}
	err := r.db.QueryRowContext(ctx, q, id).Scan(
		&m.ID, &m.UserID, &m.Name, &m.PhotoMediaID, &m.CreatedAt, &m.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, medicine.ErrNotFound
	}
	if err != nil {
		return nil, apperrors.DB("medicinerepo.GetByID", err)
	}
	return m, nil
}

// ListByUser возвращает все лекарства пользователя.
func (r *Repository) ListByUser(ctx context.Context, userID int64) ([]medicine.Medicine, error) {
	const q = `
		SELECT id, user_id, name, photo_media_id, created_at, updated_at
		FROM medicines
		WHERE user_id = $1
		ORDER BY created_at ASC`

	rows, err := r.db.QueryContext(ctx, q, userID)
	if err != nil {
		return nil, apperrors.DB("medicinerepo.ListByUser", err)
	}
	defer rows.Close()

	var result []medicine.Medicine
	for rows.Next() {
		var m medicine.Medicine
		if err = rows.Scan(
			&m.ID, &m.UserID, &m.Name, &m.PhotoMediaID, &m.CreatedAt, &m.UpdatedAt,
		); err != nil {
			return nil, apperrors.DB("medicinerepo.ListByUser.scan", err)
		}
		result = append(result, m)
	}
	if err = rows.Err(); err != nil {
		return nil, apperrors.DB("medicinerepo.ListByUser.rows", err)
	}
	return result, nil
}

// Create создаёт новое лекарство.
func (r *Repository) Create(ctx context.Context, m *medicine.Medicine) error {
	const q = `
		INSERT INTO medicines (user_id, name, photo_media_id)
		VALUES ($1, $2, $3)
		RETURNING id, created_at, updated_at`

	err := r.db.QueryRowContext(ctx, q, m.UserID, m.Name, m.PhotoMediaID).
		Scan(&m.ID, &m.CreatedAt, &m.UpdatedAt)
	if err != nil {
		return apperrors.DB("medicinerepo.Create", err)
	}
	return nil
}

// Update обновляет имя и фото лекарства.
func (r *Repository) Update(ctx context.Context, m *medicine.Medicine) error {
	const q = `
		UPDATE medicines
		SET name = $1, photo_media_id = $2, updated_at = now()
		WHERE id = $3
		RETURNING updated_at`

	err := r.db.QueryRowContext(ctx, q, m.Name, m.PhotoMediaID, m.ID).
		Scan(&m.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return medicine.ErrNotFound
	}
	if err != nil {
		return apperrors.DB("medicinerepo.Update", err)
	}
	return nil
}

// Delete удаляет лекарство.
func (r *Repository) Delete(ctx context.Context, id int64) error {
	const q = `DELETE FROM medicines WHERE id = $1`

	res, err := r.db.ExecContext(ctx, q, id)
	if err != nil {
		return apperrors.DB("medicinerepo.Delete", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return medicine.ErrNotFound
	}
	return nil
}
