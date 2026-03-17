package reminder

import (
	"time"

	"github.com/google/uuid"
)

type Status string

const (
	// StatusPending — ожидает отправки.
	StatusPending Status = "pending"

	// StatusSent — отправлено пользователю, ожидает реакции.
	StatusSent Status = "sent"

	// StatusConfirmed — пользователь подтвердил приём.
	StatusConfirmed Status = "confirmed"

	// StatusSkipped — пользователь пропустил приём.
	StatusSkipped Status = "skipped"

	// StatusPostponed — пользователь отложил напоминание.
	StatusPostponed Status = "postponed"

	// StatusExpired — время реакции истекло.
	StatusExpired Status = "expired"
)

type Reminder struct {
	ID             int64
	UserID         int64
	MedicineID     int64
	ScheduleID     int64
	ScheduledAt    time.Time
	Status         Status
	RetryCount     int16
	PostponeCount  int16
	IdempotencyKey uuid.UUID
	CreatedAt      time.Time
}

func (r *Reminder) IsFinal() bool {
	switch r.Status {
	case StatusConfirmed, StatusSkipped, StatusExpired:
		return true
	default:
		return false
	}
}

func (r *Reminder) CanRetry(maxRetries int) bool {
	return !r.IsFinal() && int(r.RetryCount) < maxRetries
}
