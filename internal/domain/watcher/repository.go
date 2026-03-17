package watcher

import (
	"context"
	"time"
)

type Repository interface {
	// GetByID возвращает наблюдателя по ID.
	GetByID(ctx context.Context, id int64) (*Watcher, error)

	// GetByPair возвращает наблюдателя по паре (userID, watcherUserID).
	GetByPair(ctx context.Context, userID, watcherUserID int64) (*Watcher, error)

	// ListByUser возвращает всех наблюдателей пациента.
	ListByUser(ctx context.Context, userID int64) ([]Watcher, error)

	// ListByWatcher возвращает всех пациентов которых наблюдает watcherUserID.
	ListByWatcher(ctx context.Context, watcherUserID int64) ([]Watcher, error)

	// Create добавляет нового наблюдателя.
	Create(ctx context.Context, watcher *Watcher) error

	// Delete удаляет наблюдателя (каскадно удаляет уведомления).
	Delete(ctx context.Context, id int64) error

	// GetNotificationByID возвращает уведомление по ID.
	GetNotificationByID(ctx context.Context, id int64) (*WatcherNotification, error)

	// ListPendingNotifications возвращает неотправленные уведомления.
	ListPendingNotifications(ctx context.Context, limit int) ([]WatcherNotification, error)

	// CreateNotification создаёт уведомление для наблюдателя.
	CreateNotification(ctx context.Context, notification *WatcherNotification) error

	// MarkNotificationSent помечает уведомление как доставленное.
	MarkNotificationSent(ctx context.Context, id int64, sentAt time.Time) error
}
