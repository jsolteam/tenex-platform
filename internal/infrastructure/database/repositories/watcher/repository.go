package watcherrepo

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/lib/pq"
	"go.uber.org/zap"

	"github.com/jsolteam/tenex-platform/internal/domain/watcher"
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
		log:    log.With(zap.String("repo", "watcher")),
		tracer: tracer,
	}
}

func (r *Repository) GetByID(ctx context.Context, id int64) (*watcher.Watcher, error) {
	ctx, span := r.tracer.Start(ctx, "watcherrepo.GetByID")
	defer span.End()

	w := &watcher.Watcher{}
	err := r.db.QueryRowContext(ctx, `SELECT id, user_id, watcher_user_id, created_at FROM watchers WHERE id=$1`, id).
		Scan(&w.ID, &w.UserID, &w.WatcherUserID, &w.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		repolog.Debug(ctx, r.log, "watcher not found", zap.Int64("id", id))
		return nil, watcher.ErrNotFound
	}
	if err != nil {
		appErr := apperrors.DB("watcherrepo.GetByID", err)
		repolog.Err(ctx, r.log, span, "failed to get watcher", appErr, zap.Int64("id", id))
		return nil, appErr
	}
	return w, nil
}

func (r *Repository) GetByPair(ctx context.Context, userID, watcherUserID int64) (*watcher.Watcher, error) {
	ctx, span := r.tracer.Start(ctx, "watcherrepo.GetByPair")
	defer span.End()

	w := &watcher.Watcher{}
	err := r.db.QueryRowContext(ctx, `SELECT id, user_id, watcher_user_id, created_at FROM watchers WHERE user_id=$1 AND watcher_user_id=$2`, userID, watcherUserID).
		Scan(&w.ID, &w.UserID, &w.WatcherUserID, &w.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		repolog.Debug(ctx, r.log, "watcher pair not found",
			zap.Int64("user_id", userID), zap.Int64("watcher_user_id", watcherUserID))
		return nil, watcher.ErrNotFound
	}
	if err != nil {
		appErr := apperrors.DB("watcherrepo.GetByPair", err)
		repolog.Err(ctx, r.log, span, "failed to get watcher by pair", appErr, zap.Int64("user_id", userID))
		return nil, appErr
	}
	return w, nil
}

func (r *Repository) ListByUser(ctx context.Context, userID int64) ([]watcher.Watcher, error) {
	ctx, span := r.tracer.Start(ctx, "watcherrepo.ListByUser")
	defer span.End()

	result, err := r.queryWatchers(ctx, `SELECT id, user_id, watcher_user_id, created_at FROM watchers WHERE user_id=$1 ORDER BY created_at ASC`, userID)
	if err != nil {
		appErr := apperrors.DB("watcherrepo.ListByUser", err)
		repolog.Err(ctx, r.log, span, "failed to list watchers", appErr, zap.Int64("user_id", userID))
		return nil, appErr
	}
	return result, nil
}

func (r *Repository) ListByWatcher(ctx context.Context, watcherUserID int64) ([]watcher.Watcher, error) {
	ctx, span := r.tracer.Start(ctx, "watcherrepo.ListByWatcher")
	defer span.End()

	result, err := r.queryWatchers(ctx, `SELECT id, user_id, watcher_user_id, created_at FROM watchers WHERE watcher_user_id=$1 ORDER BY created_at ASC`, watcherUserID)
	if err != nil {
		appErr := apperrors.DB("watcherrepo.ListByWatcher", err)
		repolog.Err(ctx, r.log, span, "failed to list watched users", appErr, zap.Int64("watcher_user_id", watcherUserID))
		return nil, appErr
	}
	return result, nil
}

func (r *Repository) Create(ctx context.Context, w *watcher.Watcher) error {
	ctx, span := r.tracer.Start(ctx, "watcherrepo.Create")
	defer span.End()

	err := r.db.QueryRowContext(ctx, `INSERT INTO watchers (user_id, watcher_user_id) VALUES ($1,$2) RETURNING id, created_at`, w.UserID, w.WatcherUserID).
		Scan(&w.ID, &w.CreatedAt)
	if err != nil {
		if isUniqueViolation(err) {
			appErr := apperrors.Validation("watcherrepo.Create", watcher.ErrAlreadyExists)
			repolog.Err(ctx, r.log, span, "watcher pair already exists", appErr,
				zap.Int64("user_id", w.UserID), zap.Int64("watcher_user_id", w.WatcherUserID))
			return appErr
		}
		appErr := apperrors.DB("watcherrepo.Create", err)
		repolog.Err(ctx, r.log, span, "failed to create watcher", appErr, zap.Int64("user_id", w.UserID))
		return appErr
	}
	return nil
}

func (r *Repository) Delete(ctx context.Context, id int64) error {
	ctx, span := r.tracer.Start(ctx, "watcherrepo.Delete")
	defer span.End()

	res, err := r.db.ExecContext(ctx, `DELETE FROM watchers WHERE id=$1`, id)
	if err != nil {
		appErr := apperrors.DB("watcherrepo.Delete", err)
		repolog.Err(ctx, r.log, span, "failed to delete watcher", appErr, zap.Int64("id", id))
		return appErr
	}
	if n, _ := res.RowsAffected(); n == 0 {
		repolog.Debug(ctx, r.log, "watcher not found on delete", zap.Int64("id", id))
		return watcher.ErrNotFound
	}
	return nil
}

func (r *Repository) GetNotificationByID(ctx context.Context, id int64) (*watcher.WatcherNotification, error) {
	ctx, span := r.tracer.Start(ctx, "watcherrepo.GetNotificationByID")
	defer span.End()

	n, err := scanNotification(r.db.QueryRowContext(ctx, `SELECT id, watcher_id, reminder_id, event, sent_at, created_at FROM watcher_notifications WHERE id=$1`, id))
	if errors.Is(err, sql.ErrNoRows) {
		repolog.Debug(ctx, r.log, "watcher notification not found", zap.Int64("id", id))
		return nil, watcher.ErrNotificationNotFound
	}
	if err != nil {
		appErr := apperrors.DB("watcherrepo.GetNotificationByID", err)
		repolog.Err(ctx, r.log, span, "failed to get watcher notification", appErr, zap.Int64("id", id))
		return nil, appErr
	}
	return n, nil
}

func (r *Repository) ListPendingNotifications(ctx context.Context, limit int) ([]watcher.WatcherNotification, error) {
	ctx, span := r.tracer.Start(ctx, "watcherrepo.ListPendingNotifications")
	defer span.End()

	rows, err := r.db.QueryContext(ctx, `SELECT id, watcher_id, reminder_id, event, sent_at, created_at FROM watcher_notifications WHERE sent_at IS NULL ORDER BY created_at ASC LIMIT $1`, limit)
	if err != nil {
		appErr := apperrors.DB("watcherrepo.ListPendingNotifications", err)
		repolog.Err(ctx, r.log, span, "failed to list pending notifications", appErr)
		return nil, appErr
	}
	defer rows.Close()

	var result []watcher.WatcherNotification
	for rows.Next() {
		n, err := scanNotification(rows)
		if err != nil {
			appErr := apperrors.DB("watcherrepo.ListPendingNotifications.scan", err)
			repolog.Err(ctx, r.log, span, "failed to scan notification", appErr)
			return nil, appErr
		}
		result = append(result, *n)
	}
	if err = rows.Err(); err != nil {
		appErr := apperrors.DB("watcherrepo.ListPendingNotifications.rows", err)
		repolog.Err(ctx, r.log, span, "notifications rows error", appErr)
		return nil, appErr
	}
	return result, nil
}

func (r *Repository) CreateNotification(ctx context.Context, n *watcher.WatcherNotification) error {
	ctx, span := r.tracer.Start(ctx, "watcherrepo.CreateNotification")
	defer span.End()

	err := r.db.QueryRowContext(ctx, `INSERT INTO watcher_notifications (watcher_id, reminder_id, event) VALUES ($1,$2,$3) RETURNING id, created_at`, n.WatcherID, n.ReminderID, n.Event).
		Scan(&n.ID, &n.CreatedAt)
	if err != nil {
		appErr := apperrors.DB("watcherrepo.CreateNotification", err)
		repolog.Err(ctx, r.log, span, "failed to create watcher notification", appErr,
			zap.Int64("watcher_id", n.WatcherID), zap.String("event", string(n.Event)))
		return appErr
	}
	return nil
}

func (r *Repository) MarkNotificationSent(ctx context.Context, id int64, sentAt time.Time) error {
	ctx, span := r.tracer.Start(ctx, "watcherrepo.MarkNotificationSent")
	defer span.End()

	res, err := r.db.ExecContext(ctx, `UPDATE watcher_notifications SET sent_at=$1 WHERE id=$2 AND sent_at IS NULL`, sentAt, id)
	if err != nil {
		appErr := apperrors.DB("watcherrepo.MarkNotificationSent", err)
		repolog.Err(ctx, r.log, span, "failed to mark notification sent", appErr, zap.Int64("id", id))
		return appErr
	}
	if n, _ := res.RowsAffected(); n == 0 {
		repolog.Debug(ctx, r.log, "notification already sent or not found", zap.Int64("id", id))
		return watcher.ErrNotificationAlreadySent
	}
	return nil
}

// ── helpers ───────────────────────────────────────────────────────────────

type rowScanner interface{ Scan(dest ...any) error }

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

func scanNotification(s rowScanner) (*watcher.WatcherNotification, error) {
	n := &watcher.WatcherNotification{}
	var sentAt sql.NullTime
	if err := s.Scan(&n.ID, &n.WatcherID, &n.ReminderID, &n.Event, &sentAt, &n.CreatedAt); err != nil {
		return nil, err
	}
	if sentAt.Valid {
		n.SentAt = &sentAt.Time
	}
	return n, nil
}

func isUniqueViolation(err error) bool {
	var pqErr *pq.Error
	return errors.As(err, &pqErr) && pqErr.Code == "23505"
}
