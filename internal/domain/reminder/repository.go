package reminder

import (
	"context"
	"time"

	"github.com/google/uuid"
)

type Repository interface {
	// GetByID возвращает напоминание по ID и scheduled_at.
	GetByID(ctx context.Context, id int64, scheduledAt time.Time) (*Reminder, error)

	// GetByIdempotencyKey возвращает напоминание по ключу идемпотентности.
	GetByIdempotencyKey(ctx context.Context, key uuid.UUID) (*Reminder, error)

	// ListByUser возвращает историю напоминаний пользователя.
	ListByUser(ctx context.Context, userID int64, from, to time.Time) ([]Reminder, error)

	// ListPending возвращает напоминания в статусе pending/sent
	ListPending(ctx context.Context, before time.Time, limit int) ([]Reminder, error)

	// ListBySchedule возвращает напоминания для конкретного расписания.
	ListBySchedule(ctx context.Context, scheduleID int64, from, to time.Time) ([]Reminder, error)

	// Create создаёт новое напоминание.
	Create(ctx context.Context, reminder *Reminder) error

	// UpdateStatus обновляет статус напоминания.
	UpdateStatus(ctx context.Context, id int64, scheduledAt time.Time, status Status) error

	// IncrementRetry увеличивает счётчик ретраев.
	IncrementRetry(ctx context.Context, id int64, scheduledAt time.Time) error

	// IncrementPostpone увеличивает счётчик откладываний.
	IncrementPostpone(ctx context.Context, id int64, scheduledAt time.Time) error
}
