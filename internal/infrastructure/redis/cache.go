package redis

import (
	"context"
	"errors"
	"time"

	goredis "github.com/redis/go-redis/v9"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.uber.org/zap"

	apperrors "github.com/jsolteam/tenex-platform/internal/platform/errors"
	contextlog "github.com/jsolteam/tenex-platform/internal/platform/logger/context"
)

// ErrCacheMiss возвращается когда ключ не найден в кэше.
var ErrCacheMiss = errors.New("cache: miss")

// Cache — Redis-кэш с byte-значениями.
// Сериализацию данных выполняет вызывающий код.
type Cache struct {
	client *Client
}

// NewCache создаёт Cache поверх существующего Client.
func NewCache(c *Client) *Cache {
	return &Cache{client: c}
}

// Get возвращает значение по ключу.
// Возвращает ErrCacheMiss если ключ не существует или истёк TTL.
func (c *Cache) Get(ctx context.Context, key string) ([]byte, error) {
	ctx, span := c.client.tracer.Start(ctx, "cache.Get")
	defer span.End()
	span.SetAttributes(attribute.String("cache.key", key))

	val, err := c.client.rdb.Get(ctx, key).Bytes()
	if errors.Is(err, goredis.Nil) {
		span.SetAttributes(attribute.Bool("cache.hit", false))
		contextlog.FromCtx(ctx, c.client.log).Debug("cache miss", zap.String("key", key))
		return nil, ErrCacheMiss
	}
	if err != nil {
		appErr := apperrors.Redis("cache.Get", err)
		span.RecordError(appErr)
		span.SetStatus(codes.Error, string(appErr.Code))
		contextlog.FromCtx(ctx, c.client.log).Error("cache get failed",
			zap.Error(appErr),
			zap.String("key", key),
		)
		return nil, appErr
	}

	span.SetAttributes(attribute.Bool("cache.hit", true))
	return val, nil
}

// Set сохраняет значение с TTL.
// ttl=0 означает хранение без срока истечения.
func (c *Cache) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	ctx, span := c.client.tracer.Start(ctx, "cache.Set")
	defer span.End()
	span.SetAttributes(
		attribute.String("cache.key", key),
		attribute.Int("cache.value_bytes", len(value)),
	)

	if err := c.client.rdb.Set(ctx, key, value, ttl).Err(); err != nil {
		appErr := apperrors.Redis("cache.Set", err)
		span.RecordError(appErr)
		span.SetStatus(codes.Error, string(appErr.Code))
		contextlog.FromCtx(ctx, c.client.log).Error("cache set failed",
			zap.Error(appErr),
			zap.String("key", key),
		)
		return appErr
	}
	return nil
}

// Delete удаляет ключ из кэша.
// Если ключа не существует — не возвращает ошибку.
func (c *Cache) Delete(ctx context.Context, keys ...string) error {
	if len(keys) == 0 {
		return nil
	}

	ctx, span := c.client.tracer.Start(ctx, "cache.Delete")
	defer span.End()
	span.SetAttributes(attribute.Int("cache.keys_count", len(keys)))

	if err := c.client.rdb.Del(ctx, keys...).Err(); err != nil {
		appErr := apperrors.Redis("cache.Delete", err)
		span.RecordError(appErr)
		span.SetStatus(codes.Error, string(appErr.Code))
		contextlog.FromCtx(ctx, c.client.log).Error("cache delete failed",
			zap.Error(appErr),
			zap.Strings("keys", keys),
		)
		return appErr
	}
	return nil
}

// Exists проверяет существование ключа.
func (c *Cache) Exists(ctx context.Context, key string) (bool, error) {
	ctx, span := c.client.tracer.Start(ctx, "cache.Exists")
	defer span.End()
	span.SetAttributes(attribute.String("cache.key", key))

	n, err := c.client.rdb.Exists(ctx, key).Result()
	if err != nil {
		appErr := apperrors.Redis("cache.Exists", err)
		span.RecordError(appErr)
		span.SetStatus(codes.Error, string(appErr.Code))
		contextlog.FromCtx(ctx, c.client.log).Error("cache exists failed",
			zap.Error(appErr),
			zap.String("key", key),
		)
		return false, appErr
	}
	return n > 0, nil
}
