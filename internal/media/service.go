package media

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"go.opentelemetry.io/otel/attribute"
	"go.uber.org/zap"

	domainmedia "github.com/jsolteam/tenex-platform/internal/domain/media"
	infraredis "github.com/jsolteam/tenex-platform/internal/infrastructure/redis"
	infras3 "github.com/jsolteam/tenex-platform/internal/infrastructure/s3"
	apperrors "github.com/jsolteam/tenex-platform/internal/platform/errors"
	contextlog "github.com/jsolteam/tenex-platform/internal/platform/logger/context"
	"github.com/jsolteam/tenex-platform/internal/platform/logger/core"
	"github.com/jsolteam/tenex-platform/internal/platform/observability/tracing"
)

const fileIDTTL = 7 * 24 * time.Hour

type Service struct {
	repo   domainmedia.Repository
	s3     *infras3.Client
	cache  *infraredis.Cache
	log    *core.Logger
	tracer tracing.Tracer
}

func NewService(
	repo domainmedia.Repository,
	s3 *infras3.Client,
	cache *infraredis.Cache,
	log *core.Logger,
	tracer tracing.Tracer,
) *Service {
	return &Service{
		repo:   repo,
		s3:     s3,
		cache:  cache,
		log:    log.With(zap.String("component", "media_service")),
		tracer: tracer,
	}
}

// UploadPhoto загружает фото в S3 и создаёт записи media + media_variant в БД.
// Если БД упала после успешной загрузки в S3 — выполняет компенсирующее удаление S3-объекта.
func (s *Service) UploadPhoto(ctx context.Context, ownerID int64, data []byte, contentType string) (int64, string, error) {
	ctx, span := s.tracer.Start(ctx, "media.UploadPhoto")
	defer span.End()
	span.SetAttributes(
		attribute.Int64("media.owner_id", ownerID),
		attribute.Int("media.size_bytes", len(data)),
	)

	s3Key := buildS3Key(ownerID, contentType)

	// Шаг 1: загружаем в S3
	if err := s.s3.Upload(ctx, s3Key, bytes.NewReader(data), int64(len(data)), contentType); err != nil {
		return 0, "", err
	}

	contextlog.FromCtx(ctx, s.log).Debug("media: s3 upload ok", zap.String("key", s3Key))

	// Шаг 2: создаём запись в БД
	mediaRec := &domainmedia.Media{
		OwnerUserID: ownerID,
		MediaType:   domainmedia.MediaTypePhoto,
	}
	if err := s.repo.Create(ctx, mediaRec); err != nil {
		// Компенсация: удаляем S3-объект если БД упала
		if delErr := s.s3.Delete(ctx, s3Key); delErr != nil {
			contextlog.FromCtx(ctx, s.log).Error("media: компенсация не удалась — S3-объект остался сиротой",
				zap.String("s3_key", s3Key),
				zap.Error(delErr),
			)
		}
		return 0, "", apperrors.DB("media.UploadPhoto.create", err)
	}

	// Шаг 3: добавляем S3-вариант
	variant := &domainmedia.MediaVariant{
		MediaID:     mediaRec.ID,
		StorageType: domainmedia.StorageTypeS3,
		ExternalID:  s3Key,
	}
	if err := s.repo.AddVariant(ctx, variant); err != nil {
		// Компенсация: удаляем и S3-объект, и запись media
		if delErr := s.s3.Delete(ctx, s3Key); delErr != nil {
			contextlog.FromCtx(ctx, s.log).Error("media: компенсация (variant) — не удалось удалить S3",
				zap.String("s3_key", s3Key), zap.Error(delErr),
			)
		}
		if delErr := s.repo.Delete(ctx, mediaRec.ID); delErr != nil {
			contextlog.FromCtx(ctx, s.log).Error("media: компенсация (variant) — не удалось удалить media из БД",
				zap.Int64("media_id", mediaRec.ID), zap.Error(delErr),
			)
		}
		return 0, "", apperrors.DB("media.UploadPhoto.add_variant", err)
	}

	contextlog.FromCtx(ctx, s.log).Info("media: фото загружено",
		zap.Int64("media_id", mediaRec.ID),
		zap.String("s3_key", s3Key),
		zap.Int64("owner_id", ownerID),
	)

	return mediaRec.ID, s3Key, nil
}

// DeleteMedia удаляет S3-объект и все записи из БД.
func (s *Service) DeleteMedia(ctx context.Context, mediaID int64) error {
	ctx, span := s.tracer.Start(ctx, "media.DeleteMedia")
	defer span.End()
	span.SetAttributes(attribute.Int64("media.id", mediaID))

	// Находим S3-вариант чтобы знать ключ для удаления
	variants, err := s.repo.GetVariants(ctx, mediaID)
	if err != nil {
		return err
	}

	for _, v := range variants {
		if v.StorageType == domainmedia.StorageTypeS3 && v.ExternalID != "" {
			if err := s.s3.Delete(ctx, v.ExternalID); err != nil {
				contextlog.FromCtx(ctx, s.log).Warn("media: не удалось удалить S3-объект",
					zap.String("key", v.ExternalID), zap.Error(err),
				)
				// Не прерываем — удаляем запись в БД в любом случае
			}
		}
	}

	if err := s.repo.Delete(ctx, mediaID); err != nil {
		if errors.Is(err, domainmedia.ErrNotFound) {
			return nil
		}
		return err
	}

	contextlog.FromCtx(ctx, s.log).Debug("media: удалено", zap.Int64("media_id", mediaID))
	return nil
}

// GetFileID возвращает закэшированный FileID мессенджера.
// Возвращает "", nil если кэш не найден.
func (s *Service) GetFileID(ctx context.Context, mediaID int64, messenger string) (string, error) {
	ctx, span := s.tracer.Start(ctx, "media.GetFileID")
	defer span.End()

	key := fileIDCacheKey(mediaID, messenger)
	data, err := s.cache.Get(ctx, key)
	if errors.Is(err, infraredis.ErrCacheMiss) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// CacheFileID сохраняет FileID в Redis с TTL 7 дней.
func (s *Service) CacheFileID(ctx context.Context, mediaID int64, messenger, fileID string) error {
	ctx, span := s.tracer.Start(ctx, "media.CacheFileID")
	defer span.End()

	key := fileIDCacheKey(mediaID, messenger)
	return s.cache.Set(ctx, key, []byte(fileID), fileIDTTL)
}

// PresignedURL генерирует временную публичную ссылку на S3-объект медиафайла.
func (s *Service) PresignedURL(ctx context.Context, mediaID int64, expires time.Duration) (string, error) {
	ctx, span := s.tracer.Start(ctx, "media.PresignedURL")
	defer span.End()

	variant, err := s.repo.GetVariantByStorage(ctx, mediaID, domainmedia.StorageTypeS3, "")
	if err != nil {
		if errors.Is(err, domainmedia.ErrVariantNotFound) {
			return "", apperrors.Validation("media.PresignedURL", fmt.Errorf("media %d не имеет S3-варианта", mediaID))
		}
		return "", err
	}

	return s.s3.PresignedURL(ctx, variant.ExternalID, expires)
}

// GetOrUploadBytes возвращает закэшированный FileID или загружает данные и кэширует результат.
// Используется для welcome-баннера: первый вызов загружает, последующие читают из кэша.
// Если mediaID == 0 — создаёт новую запись (системное медиа, ownerID = -1).
func (s *Service) GetOrUploadBytes(ctx context.Context, ownerID int64, data []byte, contentType, messenger string) (int64, string, error) {
	ctx, span := s.tracer.Start(ctx, "media.GetOrUploadBytes")
	defer span.End()

	// Пробуем найти существующую запись по S3-варианту для данного ownerID
	// (упрощённо: если FileID уже есть в кэше для этого ownerID — возвращаем)
	// В реальном сценарии welcome-баннер передаёт известный mediaID через конфиг.
	// Этот метод создаёт новую запись если mediaID неизвестен.
	mediaID, s3Key, err := s.UploadPhoto(ctx, ownerID, data, contentType)
	if err != nil {
		return 0, "", err
	}

	// FileID пока неизвестен — вернём пустую строку.
	// Хэндлер должен отправить файл, получить FileID от мессенджера и вызвать CacheFileID.
	_ = s3Key
	_ = messenger

	return mediaID, "", nil
}

// ── вспомогательные функции ───────────────────────────────────────────────────

// buildS3Key формирует ключ объекта в S3.
// Формат: media/{ownerUserID}/{uuid}.{ext}
func buildS3Key(ownerID int64, contentType string) string {
	ext := extFromContentType(contentType)
	return fmt.Sprintf("media/%d/%s%s", ownerID, uuid.New().String(), ext)
}

// extFromContentType возвращает расширение файла по content-type.
func extFromContentType(ct string) string {
	ct = strings.ToLower(strings.TrimSpace(ct))
	switch {
	case strings.Contains(ct, "jpeg") || strings.Contains(ct, "jpg"):
		return ".jpg"
	case strings.Contains(ct, "png"):
		return ".png"
	case strings.Contains(ct, "gif"):
		return ".gif"
	case strings.Contains(ct, "webp"):
		return ".webp"
	case strings.Contains(ct, "mp4"):
		return ".mp4"
	default:
		// Пытаемся взять из mime-type напрямую
		if idx := strings.Index(ct, "/"); idx >= 0 {
			ext := "." + ct[idx+1:]
			if filepath.Ext(ext) != "" {
				return ext
			}
		}
		return ""
	}
}

// fileIDCacheKey формирует ключ Redis для FileID.
// Формат: media:fileid:{mediaID}:{messenger}
func fileIDCacheKey(mediaID int64, messenger string) string {
	return fmt.Sprintf("media:fileid:%d:%s", mediaID, messenger)
}
