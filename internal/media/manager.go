package media

import (
	"context"
	"time"
)

// Manager управляет жизненным циклом медиафайлов:
// загрузка в S3, запись метаданных в БД, кэширование FileID мессенджеров.
type Manager interface {
	// UploadPhoto загружает фото в S3 и сохраняет метаданные в БД.
	// Возвращает ID созданной записи media и S3-ключ.
	// Компенсация: если БД упала после S3-загрузки — S3-объект удаляется.
	UploadPhoto(ctx context.Context, ownerID int64, data []byte, contentType string) (mediaID int64, s3Key string, err error)

	// DeleteMedia удаляет S3-объект и все записи из БД (media + variants).
	// Если S3-объект уже отсутствует — не возвращает ошибку.
	DeleteMedia(ctx context.Context, mediaID int64) error

	// GetFileID возвращает закэшированный FileID мессенджера для медиафайла.
	// Возвращает "", nil если кэш отсутствует.
	GetFileID(ctx context.Context, mediaID int64, messenger string) (fileID string, err error)

	// CacheFileID сохраняет FileID мессенджера в Redis.
	// Ключ: media:fileid:{mediaID}:{messenger}, TTL 7 дней.
	CacheFileID(ctx context.Context, mediaID int64, messenger, fileID string) error

	// PresignedURL генерирует временную публичную ссылку на S3-объект.
	PresignedURL(ctx context.Context, mediaID int64, expires time.Duration) (string, error)

	// GetOrUploadBytes возвращает закэшированный FileID если он есть,
	// иначе загружает данные в S3, создаёт запись в БД и кэширует FileID.
	GetOrUploadBytes(ctx context.Context, ownerID int64, data []byte, contentType, messenger string) (mediaID int64, fileID string, err error)
}
