package schedule

import (
	"context"
	"time"
)

type Repository interface {
	// GetByID возвращает расписание по ID.
	GetByID(ctx context.Context, id int64) (*Schedule, error)

	// ListByMedicine возвращает все расписания для лекарства.
	ListByMedicine(ctx context.Context, medicineID int64) ([]Schedule, error)

	// ListActive возвращает расписания активные на указанную дату.
	ListActive(ctx context.Context, date time.Time) ([]Schedule, error)

	// Create создаёт новое расписание.
	Create(ctx context.Context, schedule *Schedule) error

	// Update обновляет расписание.
	Update(ctx context.Context, schedule *Schedule) error

	// Delete удаляет расписание.
	Delete(ctx context.Context, id int64) error
}
