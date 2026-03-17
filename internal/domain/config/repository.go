package config

import "context"

type Repository interface {
	// Get возвращает значение по ключу.
	Get(ctx context.Context, key string) (*PlatformConfig, error)

	// GetAll возвращает все записи конфигурации.
	GetAll(ctx context.Context) ([]PlatformConfig, error)

	// Set создаёт или обновляет значение конфигурации.
	Set(ctx context.Context, key, value string) error

	// Delete удаляет запись конфигурации.
	Delete(ctx context.Context, key string) error
}
