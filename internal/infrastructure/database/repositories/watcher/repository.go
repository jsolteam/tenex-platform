package watcherrepo

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/lib/pq"

	"github.com/jsolteam/tenex-platform/internal/domain/watcher"
	apperrors "github.com/jsolteam/tenex-platform/internal/platform/errors"
)

type Repository struct {
	db *sql.DB
}

func New(db *sql.DB) *Repository {
	return &Repository{db: db}
}

// GetByID возвращает наблюдателя по ID.
func (r *Repository) GetByID(ctx context.Context, id int64) (*watcher.Watcher, error) {
	const q = `SELECT id, user_id, watcher_user_id, created_at FROM watchers WHERE id = $1`

	w := &watcher.Watcher{}
	err := r.db.QueryRowContext(ctx, q, id).Scan(&w.ID, &w.UserID, &w.WatcherUserID, &w.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, watcher.ErrNotFound
	}
	if err != nil {
		return nil, apperrors.DB("watcherrepo.GetByID", err)
	}
	return w, nil
}

// GetByPair возвращает наблюдателя по паре (userID, watcherUserID).
func (r *Repository) GetByPair(ctx context.Context, userID, watcherUserID int64) (*watcher.Watcher, error) {
	const q = `
		SELECT id, user_id, watcher_user_id, created_at
		FROM watchers
		WHERE user_id = $1 AND watcher_user_id = $2`

	w := &watcher.Watcher{}
	err := r.db.QueryRowContext(ctx, q, userID, watcherUserID).
		Scan(&w.ID, &w.UserID, &w.WatcherUserID, &w.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, watcher.ErrNotFound
	}
	if err != nil {
		return nil, apperrors.DB("watcherrepo.GetByPair", err)
	}
	return w, nil
}

// ListByUser возвращает всех наблюдателей пациента.
func (r *Repository) ListByUser(ctx context.Context, userID int64) ([]watcher.Watcher, error) {
	const q = `
		SELECT id, user_id, watcher_user_id, created_at
		FROM watchers
		WHERE user_id = $1
		ORDER BY created_at ASC`

	result, err := r.queryWatchers(ctx, q, userID)
	if err != nil {
		return nil, apperrors.DB("watcherrepo.ListByUser", err)
	}
	return result, nil
}

// ListByWatcher возвращает всех пациентов которых наблюдает watcherUserID.
func (r *Repository) ListByWatcher(ctx context.Context, watcherUserID int64) ([]watcher.Watcher, error) {
	const q = `
		SELECT id, user_id, watcher_user_id, created_at
		FROM watchers
		WHERE watcher_user_id = $1
		ORDER BY created_at ASC`

	result, err := r.queryWatchers(ctx, q, watcherUserID)
	if err != nil {
		return nil, apperrors.DB("watcherrepo.ListByWatcher", err)
	}
	return result, nil
}

// Create добавляет нового наблюдателя.
func (r *Repository) Create(ctx context.Context, w *watcher.Watcher) error {
	const q = `
		INSERT INTO watchers (user_id, watcher_user_id)
		VALUES ($1, $2)
		RETURNING id, created_at`

	err := r.db.QueryRowContext(ctx, q, w.UserID, w.WatcherUserID).
		Scan(&w.ID, &w.CreatedAt)
	if err != nil {
		if isUniqueViolation(err) {
			// Пара (user_id, watcher_user_id) уже существует — нарушение uq_watchers_pair.
			return apperrors.Validation("watcherrepo.Create", watcher.ErrAlreadyExists)
		}
		return apperrors.DB("watcherrepo.Create", err)
	}
	return nil
}

// Delete удаляет наблюдателя.
func (r *Repository) Delete(ctx context.Context, id int64) error {
	const q = `DELETE FROM watchers WHERE id = $1`

	res, err := r.db.ExecContext(ctx, q, id)
	if err != nil {
		return apperrors.DB("watcherrepo.Delete", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return watcher.ErrNotFound
	}
	return nil
}

// GetNotificationByID возвращает уведомление по ID.
func (r *Repository) GetNotificationByID(ctx context.Context, id int64) (*watcher.WatcherNotification, error) {
	const q = `
		SELECT id, watcher_id, reminder_id, event, sent_at, created_at
		FROM watcher_notifications
		WHERE id = $1`

	n, err := scanNotification(r.db.QueryRowContext(ctx, q, id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, watcher.ErrNotificationNotFound
	}
	if err != nil {
		return nil, apperrors.DB("watcherrepo.GetNotificationByID", err)
	}
	return n, nil
}

// ListPendingNotifications возвращает неотправленные уведомления.
func (r *Repository) ListPendingNotifications(ctx context.Context, limit int) ([]watcher.WatcherNotification, error) {
	const q = `
		SELECT id, watcher_id, reminder_id, event, sent_at, created_at
		FROM watcher_notifications
		WHERE sent_at IS NULL
		ORDER BY created_at ASC
		LIMIT $1`

	rows, err := r.db.QueryContext(ctx, q, limit)
	if err != nil {
		return nil, apperrors.DB("watcherrepo.ListPendingNotifications", err)
	}
	defer rows.Close()

	var result []watcher.WatcherNotification
	for rows.Next() {
		n, err := scanNotification(rows)
		if err != nil {
			return nil, apperrors.DB("watcherrepo.ListPendingNotifications.scan", err)
		}
		result = append(result, *n)
	}
	if err = rows.Err(); err != nil {
		return nil, apperrors.DB("watcherrepo.ListPendingNotifications.rows", err)
	}
	return result, nil
}

// CreateNotification создаёт уведомление для наблюдателя.
func (r *Repository) CreateNotification(ctx context.Context, n *watcher.WatcherNotification) error {
	const q = `
		INSERT INTO watcher_notifications (watcher_id, reminder_id, event)
		VALUES ($1, $2, $3)
		RETURNING id, created_at`

	err := r.db.QueryRowContext(ctx, q, n.WatcherID, n.ReminderID, n.Event).
		Scan(&n.ID, &n.CreatedAt)
	if err != nil {
		return apperrors.DB("watcherrepo.CreateNotification", err)
	}
	return nil
}

// MarkNotificationSent помечает уведомление как доставленное.
func (r *Repository) MarkNotificationSent(ctx context.Context, id int64, sentAt time.Time) error {
	const q = `UPDATE watcher_notifications SET sent_at = $1 WHERE id = $2 AND sent_at IS NULL`

	res, err := r.db.ExecContext(ctx, q, sentAt, id)
	if err != nil {
		return apperrors.DB("watcherrepo.MarkNotificationSent", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return watcher.ErrNotificationAlreadySent
	}
	return nil
}

// ── helpers ───────────────────────────────────────────────────────────────

type scanner interface {
	Scan(dest ...any) error
}

func (r *Repository) queryWatchers(ctx context.Context, q string, args ...any) ([]watcher.Watcher, error) {
	rows, err := r.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []watcher.Watcher
	for rows.Next() {
		var w watcher.Watcher
		if err = rows.Scan(&w.ID, &w.UserID, &w.WatcherUserID, &w.CreatedAt); err != nil {
			return nil, err
		}
		result = append(result, w)
	}
	return result, rows.Err()
}

func scanNotification(s scanner) (*watcher.WatcherNotification, error) {
	n := &watcher.WatcherNotification{}
	var sentAt sql.NullTime
	err := s.Scan(&n.ID, &n.WatcherID, &n.ReminderID, &n.Event, &sentAt, &n.CreatedAt)
	if err != nil {
		return nil, err
	}
	if sentAt.Valid {
		n.SentAt = &sentAt.Time
	}
	return n, nil
}

// isUniqueViolation проверяет что ошибка — нарушение уникального индекса (PostgreSQL code 23505).
func isUniqueViolation(err error) bool {
	var pqErr *pq.Error
	return errors.As(err, &pqErr) && pqErr.Code == "23505"
}
