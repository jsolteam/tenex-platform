package media

import "context"

type Repository interface {
	// GetByID возвращает медиафайл по ID.
	GetByID(ctx context.Context, id int64) (*Media, error)

	// Create создаёт запись о медиафайле.
	Create(ctx context.Context, media *Media) error

	// Delete удаляет медиафайл вместе со всеми вариантами.
	Delete(ctx context.Context, id int64) error

	// GetVariants возвращает все варианты хранения для медиафайла.
	GetVariants(ctx context.Context, mediaID int64) ([]MediaVariant, error)

	// GetVariantByStorage возвращает вариант по типу хранилища и мессенджеру.
	GetVariantByStorage(ctx context.Context, mediaID int64, storageType StorageType, messenger string) (*MediaVariant, error)

	// AddVariant добавляет новый вариант хранения.
	AddVariant(ctx context.Context, variant *MediaVariant) error

	// DeleteVariant удаляет конкретный вариант хранения.
	DeleteVariant(ctx context.Context, id int64) error
}
