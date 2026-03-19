package mediarepo

import (
	"context"
	"database/sql"
	"errors"

	"github.com/jsolteam/tenex-platform/internal/domain/media"
	apperrors "github.com/jsolteam/tenex-platform/internal/platform/errors"
)

type Repository struct {
	db *sql.DB
}

func New(db *sql.DB) *Repository {
	return &Repository{db: db}
}

// GetByID возвращает медиафайл по ID.
func (r *Repository) GetByID(ctx context.Context, id int64) (*media.Media, error) {
	const q = `
		SELECT id, owner_user_id, media_type, created_at
		FROM media
		WHERE id = $1`

	m := &media.Media{}
	err := r.db.QueryRowContext(ctx, q, id).Scan(
		&m.ID, &m.OwnerUserID, &m.MediaType, &m.CreatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, media.ErrNotFound
	}
	if err != nil {
		return nil, apperrors.DB("mediarepo.GetByID", err)
	}
	return m, nil
}

// Create создаёт запись о медиафайле.
func (r *Repository) Create(ctx context.Context, m *media.Media) error {
	const q = `
		INSERT INTO media (owner_user_id, media_type)
		VALUES ($1, $2)
		RETURNING id, created_at`

	err := r.db.QueryRowContext(ctx, q, m.OwnerUserID, m.MediaType).
		Scan(&m.ID, &m.CreatedAt)
	if err != nil {
		return apperrors.DB("mediarepo.Create", err)
	}
	return nil
}

// Delete удаляет медиафайл вместе со всеми вариантами (каскадно).
func (r *Repository) Delete(ctx context.Context, id int64) error {
	const q = `DELETE FROM media WHERE id = $1`

	res, err := r.db.ExecContext(ctx, q, id)
	if err != nil {
		return apperrors.DB("mediarepo.Delete", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return media.ErrNotFound
	}
	return nil
}

// GetVariants возвращает все варианты хранения для медиафайла.
func (r *Repository) GetVariants(ctx context.Context, mediaID int64) ([]media.MediaVariant, error) {
	const q = `
		SELECT id, media_id, storage_type, COALESCE(messenger, ''), COALESCE(external_id, ''), COALESCE(url, ''), created_at
		FROM media_variants
		WHERE media_id = $1
		ORDER BY created_at ASC`

	rows, err := r.db.QueryContext(ctx, q, mediaID)
	if err != nil {
		return nil, apperrors.DB("mediarepo.GetVariants", err)
	}
	defer rows.Close()

	var variants []media.MediaVariant
	for rows.Next() {
		var v media.MediaVariant
		if err = rows.Scan(
			&v.ID, &v.MediaID, &v.StorageType, &v.Messenger, &v.ExternalID, &v.URL, &v.CreatedAt,
		); err != nil {
			return nil, apperrors.DB("mediarepo.GetVariants.scan", err)
		}
		variants = append(variants, v)
	}
	if err = rows.Err(); err != nil {
		return nil, apperrors.DB("mediarepo.GetVariants.rows", err)
	}
	return variants, nil
}

// GetVariantByStorage возвращает вариант по типу хранилища и мессенджеру.
func (r *Repository) GetVariantByStorage(ctx context.Context, mediaID int64, storageType media.StorageType, messenger string) (*media.MediaVariant, error) {
	const q = `
		SELECT id, media_id, storage_type, COALESCE(messenger, ''), COALESCE(external_id, ''), COALESCE(url, ''), created_at
		FROM media_variants
		WHERE media_id = $1 AND storage_type = $2 AND COALESCE(messenger, '') = $3`

	v := &media.MediaVariant{}
	err := r.db.QueryRowContext(ctx, q, mediaID, storageType, messenger).Scan(
		&v.ID, &v.MediaID, &v.StorageType, &v.Messenger, &v.ExternalID, &v.URL, &v.CreatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, media.ErrVariantNotFound
	}
	if err != nil {
		return nil, apperrors.DB("mediarepo.GetVariantByStorage", err)
	}
	return v, nil
}

// AddVariant добавляет новый вариант хранения.
func (r *Repository) AddVariant(ctx context.Context, v *media.MediaVariant) error {
	const q = `
		INSERT INTO media_variants (media_id, storage_type, messenger, external_id, url)
		VALUES ($1, $2, NULLIF($3, ''), NULLIF($4, ''), NULLIF($5, ''))
		RETURNING id, created_at`

	err := r.db.QueryRowContext(ctx, q,
		v.MediaID, v.StorageType, v.Messenger, v.ExternalID, v.URL,
	).Scan(&v.ID, &v.CreatedAt)
	if err != nil {
		return apperrors.DB("mediarepo.AddVariant", err)
	}
	return nil
}

// DeleteVariant удаляет конкретный вариант хранения.
func (r *Repository) DeleteVariant(ctx context.Context, id int64) error {
	const q = `DELETE FROM media_variants WHERE id = $1`

	res, err := r.db.ExecContext(ctx, q, id)
	if err != nil {
		return apperrors.DB("mediarepo.DeleteVariant", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return media.ErrVariantNotFound
	}
	return nil
}
