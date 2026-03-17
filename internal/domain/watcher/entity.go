package watcher

import "time"

type Watcher struct {
	ID            int64
	UserID        int64
	WatcherUserID int64
	CreatedAt     time.Time
}

type NotificationEvent string

const (
	// EventConfirmed — пациент принял лекарство.
	EventConfirmed NotificationEvent = "confirmed"

	// EventSkipped — пациент пропустил приём.
	EventSkipped NotificationEvent = "skipped"

	// EventPostponed — пациент отложил напоминание.
	EventPostponed NotificationEvent = "postponed"
)

type WatcherNotification struct {
	ID         int64
	WatcherID  int64
	ReminderID int64
	Event      NotificationEvent
	SentAt     *time.Time // nil = ещё не доставлено
	CreatedAt  time.Time
}

func (n *WatcherNotification) IsSent() bool {
	return n.SentAt != nil
}
