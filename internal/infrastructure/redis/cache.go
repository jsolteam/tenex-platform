package redis

import (
	"context"
	"errors"
	"time"

	"github.com/jsolteam/tenex-platform/internal/infrastructure/infralog"
	"github.com/jsolteam/tenex-platform/internal/platform/observability/metrics"
	goredis "github.com/redis/go-redis/v9"
	"go.opentelemetry.io/otel/attribute"
	"go.uber.org/zap"

	apperrors "github.com/jsolteam/tenex-platform/internal/platform/errors"
)

// ErrCacheMiss возвращается когда ключ не найден в кэше.
var ErrCacheMiss = errors.New("cache: miss")

// Cache — Redis-кэш с byte-значениями.
// Сериализацию данных выполняет вызывающий код.
type Cache struct {
	client *Client
	met    *metrics.RedisMetrics
}

// NewCache создаёт Cache поверх существующего Client.
func NewCache(c *Client) *Cache {
	return &Cache{client: c, met: c.met}
}

// Get возвращает значение по ключу.
// Возвращает ErrCacheMiss если ключ не существует или истёк TTL.
func (c *Cache) Get(ctx context.Context, key string) ([]byte, error) {
	ctx, span := c.client.tracer.Start(ctx, "cache.Get")
	defer span.End()
	start := time.Now()
	defer func() {
		c.met.RecordDuration(ctx, metrics.RedisComponentCache, "Get", time.Since(start).Seconds())
	}()
	span.SetAttributes(attribute.String("cache.key", key))

	val, err := c.client.rdb.Get(ctx, key).Bytes()
	if errors.Is(err, goredis.Nil) {
		span.SetAttributes(attribute.Bool("cache.hit", false))
		infralog.Debug(ctx, c.client.log, "cache miss", zap.String("key", key))
		return nil, ErrCacheMiss
	}
	if err != nil {
		appErr := apperrors.Redis("cache.Get", err)
		infralog.Err(ctx, c.client.log, span, c.met, metrics.RedisComponentCache, "Get", "cache get failed",
			appErr, zap.String("key", key),
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
	start := time.Now()
	defer func() {
		c.met.RecordDuration(ctx, metrics.RedisComponentCache, "Set", time.Since(start).Seconds())
	}()
	span.SetAttributes(
		attribute.String("cache.key", key),
		attribute.Int("cache.value_bytes", len(value)),
	)

	if err := c.client.rdb.Set(ctx, key, value, ttl).Err(); err != nil {
		appErr := apperrors.Redis("cache.Set", err)
		infralog.Err(ctx, c.client.log, span, c.met,
			metrics.RedisComponentCache, "Set", "cache set failed", appErr,
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
	start := time.Now()
	defer func() {
		c.met.RecordDuration(ctx, metrics.RedisComponentCache, "Delete", time.Since(start).Seconds())
	}()
	span.SetAttributes(attribute.Int("cache.keys_count", len(keys)))

	if err := c.client.rdb.Del(ctx, keys...).Err(); err != nil {
		appErr := apperrors.Redis("cache.Delete", err)
		infralog.Err(ctx, c.client.log, span, c.met,
			metrics.RedisComponentCache, "Delete", "cache delete failed", appErr,
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
	start := time.Now()
	defer func() {
		c.met.RecordDuration(ctx, metrics.RedisComponentCache, "Exists", time.Since(start).Seconds())
	}()
	span.SetAttributes(attribute.String("cache.key", key))

	n, err := c.client.rdb.Exists(ctx, key).Result()
	if err != nil {
		appErr := apperrors.Redis("cache.Exists", err)
		infralog.Err(ctx, c.client.log, span, c.met,
			metrics.RedisComponentCache, "Exists", "cache exists failed", appErr,
			zap.String("key", key),
		)
		return false, appErr
	}
	return n > 0, nil
}
