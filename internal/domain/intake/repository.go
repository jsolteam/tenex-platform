package intake

import (
	"context"
	"time"
)

type Repository interface {
	// GetByID возвращает запись о приёме по ID.
	GetByID(ctx context.Context, id int64) (*Intake, error)

	// GetByReminderID возвращает запись о приёме для напоминания.
	GetByReminderID(ctx context.Context, reminderID int64) (*Intake, error)

	// ListByReminders возвращает записи о приёмах для нескольких напоминаний.
	ListByReminders(ctx context.Context, reminderIDs []int64) ([]Intake, error)

	// ListConfirmedWithProof возвращает подтверждённые приёмы с прикреплённым медиа.
	ListConfirmedWithProof(ctx context.Context, from, to time.Time, limit int) ([]Intake, error)

	// Create создаёт запись о приёме/пропуске.
	Create(ctx context.Context, intake *Intake) error
}
