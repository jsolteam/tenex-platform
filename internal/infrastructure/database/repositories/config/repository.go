package configrepo

import (
	"context"
	"database/sql"
	"errors"

	domainconfig "github.com/jsolteam/tenex-platform/internal/domain/config"
	apperrors "github.com/jsolteam/tenex-platform/internal/platform/errors"
)

type Repository struct {
	db *sql.DB
}

func New(db *sql.DB) *Repository {
	return &Repository{db: db}
}

// Get возвращает значение по ключу.
func (r *Repository) Get(ctx context.Context, key string) (*domainconfig.PlatformConfig, error) {
	const q = `SELECT key, value, updated_at FROM platform_config WHERE key = $1`

	c := &domainconfig.PlatformConfig{}
	err := r.db.QueryRowContext(ctx, q, key).Scan(&c.Key, &c.Value, &c.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, domainconfig.ErrNotFound
	}
	if err != nil {
		return nil, apperrors.DB("configrepo.Get", err)
	}
	return c, nil
}

// GetAll возвращает все записи конфигурации.
func (r *Repository) GetAll(ctx context.Context) ([]domainconfig.PlatformConfig, error) {
	const q = `SELECT key, value, updated_at FROM platform_config ORDER BY key ASC`

	rows, err := r.db.QueryContext(ctx, q)
	if err != nil {
		return nil, apperrors.DB("configrepo.GetAll", err)
	}
	defer rows.Close()

	var result []domainconfig.PlatformConfig
	for rows.Next() {
		var c domainconfig.PlatformConfig
		if err = rows.Scan(&c.Key, &c.Value, &c.UpdatedAt); err != nil {
			return nil, apperrors.DB("configrepo.GetAll.scan", err)
		}
		result = append(result, c)
	}
	if err = rows.Err(); err != nil {
		return nil, apperrors.DB("configrepo.GetAll.rows", err)
	}
	return result, nil
}

// Set создаёт или обновляет значение конфигурации.
func (r *Repository) Set(ctx context.Context, key, value string) error {
	const q = `
		INSERT INTO platform_config (key, value, updated_at)
		VALUES ($1, $2, now())
		ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value, updated_at = now()`

	_, err := r.db.ExecContext(ctx, q, key, value)
	if err != nil {
		return apperrors.DB("configrepo.Set", err)
	}
	return nil
}

// Delete удаляет запись конфигурации.
func (r *Repository) Delete(ctx context.Context, key string) error {
	const q = `DELETE FROM platform_config WHERE key = $1`

	res, err := r.db.ExecContext(ctx, q, key)
	if err != nil {
		return apperrors.DB("configrepo.Delete", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return domainconfig.ErrNotFound
	}
	return nil
}
