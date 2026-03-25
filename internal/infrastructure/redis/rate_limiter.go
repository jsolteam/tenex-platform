package redis

import (
	"context"
	"fmt"
	"math/rand"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.uber.org/zap"

	"github.com/jsolteam/tenex-platform/internal/infrastructure/infralog"
	apperrors "github.com/jsolteam/tenex-platform/internal/platform/errors"
	"github.com/jsolteam/tenex-platform/internal/platform/observability/metrics"
)

// RateLimiter — Redis sliding window rate limiter.
type RateLimiter struct {
	client *Client
	met    *metrics.RedisMetrics
}

// NewRateLimiter создаёт RateLimiter поверх существующего Client.
func NewRateLimiter(c *Client) *RateLimiter {
	return &RateLimiter{client: c, met: c.met}
}

// Allow проверяет и при возможности фиксирует запрос пользователя.
//
//   - messenger: "telegram", "vk" и т.д.
//   - userID:    идентификатор пользователя в мессенджере
//   - limit:     максимальное количество запросов за window
//   - window:    размер скользящего окна
//
// Возвращает true если запрос разрешён, false если лимит превышен.
func (r *RateLimiter) Allow(ctx context.Context, messenger, userID string, limit int, window time.Duration) (bool, error) {
	ctx, span := r.client.tracer.Start(ctx, "rate_limiter.Allow")
	defer span.End()
	start := time.Now()
	defer func() {
		r.met.RecordDuration(ctx, metrics.RedisComponentRateLimiter, "Allow", time.Since(start).Seconds())
	}()

	key := rateLimitKey(messenger, userID)
	now := time.Now().UnixNano()
	windowStart := now - window.Nanoseconds()

	member := fmt.Sprintf("%d_%d", now, rand.Int63()) //nolint:gosec

	span.SetAttributes(
		attribute.String("ratelimit.key", key),
		attribute.Int("ratelimit.limit", limit),
	)

	ttlMs := window.Milliseconds()
	if ttlMs < 1 {
		ttlMs = 1
	}

	// Lua-скрипт — атомарный sliding window.
	// KEYS[1]  = key
	// ARGV[1]  = windowStart (nanos)
	// ARGV[2]  = score/now   (nanos)
	// ARGV[3]  = уникальный member
	// ARGV[4]  = limit
	// ARGV[5]  = TTL в миллисекундах
	const script = `
local key        = KEYS[1]
local win_start  = tonumber(ARGV[1])
local score      = tonumber(ARGV[2])
local member     = ARGV[3]
local lim        = tonumber(ARGV[4])
local ttl_ms     = tonumber(ARGV[5])
redis.call('ZREMRANGEBYSCORE', key, '-inf', win_start)
local cnt = redis.call('ZCARD', key)
if cnt < lim then
    redis.call('ZADD', key, score, member)
    redis.call('PEXPIRE', key, ttl_ms)
    return 1
end
return 0`

	result, err := r.client.rdb.Eval(ctx, script, []string{key},
		windowStart, now, member, limit, ttlMs,
	).Int()
	if err != nil {
		appErr := apperrors.Redis("rate_limiter.Allow", err).Retryable()
		infralog.Err(ctx, r.client.log, span, r.met,
			metrics.RedisComponentRateLimiter, "Allow", "rate limiter eval failed", appErr,
			zap.String("key", key),
		)
		return false, appErr
	}

	allowed := result == 1
	span.SetAttributes(attribute.Bool("ratelimit.allowed", allowed))
	if !allowed {
		infralog.Debug(ctx, r.client.log, "rate limit exceeded",
			zap.String("key", key),
			zap.Int("limit", limit),
			zap.Duration("window", window),
		)
	}
	return allowed, nil
}

// Reset сбрасывает счётчик для пользователя.
// Используется в тестах и при ручном сбросе администратором.
func (r *RateLimiter) Reset(ctx context.Context, messenger, userID string) error {
	ctx, span := r.client.tracer.Start(ctx, "rate_limiter.Reset")
	defer span.End()
	start := time.Now()
	defer func() {
		r.met.RecordDuration(ctx, metrics.RedisComponentRateLimiter, "Reset", time.Since(start).Seconds())
	}()

	key := rateLimitKey(messenger, userID)
	if err := r.client.rdb.Del(ctx, key).Err(); err != nil {
		appErr := apperrors.Redis("rate_limiter.Reset", err)
		infralog.Err(ctx, r.client.log, span, r.met,
			metrics.RedisComponentRateLimiter, "Reset", "rate limiter reset failed", appErr,
			zap.String("key", key),
		)
		return appErr
	}
	return nil
}

func rateLimitKey(messenger, userID string) string {
	return "ratelimit:" + messenger + ":" + userID
}
