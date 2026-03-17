package medicine

import "context"

type Repository interface {
	// GetByID возвращает лекарство по ID.
	GetByID(ctx context.Context, id int64) (*Medicine, error)

	// ListByUser возвращает все лекарства пользователя.
	ListByUser(ctx context.Context, userID int64) ([]Medicine, error)

	// Create создаёт новое лекарство.
	Create(ctx context.Context, medicine *Medicine) error

	// Update обновляет имя и фото лекарства.
	Update(ctx context.Context, medicine *Medicine) error

	// Delete удаляет лекарство (каскадно удаляет расписания).
	Delete(ctx context.Context, id int64) error
}
