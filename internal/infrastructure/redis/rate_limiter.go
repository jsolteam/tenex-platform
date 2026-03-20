package redis

import (
	"context"
	"fmt"
	"math/rand"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.uber.org/zap"

	apperrors "github.com/jsolteam/tenex-platform/internal/platform/errors"
	contextlog "github.com/jsolteam/tenex-platform/internal/platform/logger/context"
)

// RateLimiter — Redis sliding window rate limiter.
type RateLimiter struct {
	client *Client
}

// NewRateLimiter создаёт RateLimiter поверх существующего Client.
func NewRateLimiter(c *Client) *RateLimiter {
	return &RateLimiter{client: c}
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
		span.RecordError(appErr)
		span.SetStatus(codes.Error, string(appErr.Code))
		contextlog.FromCtx(ctx, r.client.log).Error("rate limiter eval failed",
			zap.Error(appErr),
			zap.String("key", key),
		)
		return false, appErr
	}

	allowed := result == 1
	span.SetAttributes(attribute.Bool("ratelimit.allowed", allowed))
	if !allowed {
		contextlog.FromCtx(ctx, r.client.log).Debug("rate limit exceeded",
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

	key := rateLimitKey(messenger, userID)
	if err := r.client.rdb.Del(ctx, key).Err(); err != nil {
		appErr := apperrors.Redis("rate_limiter.Reset", err)
		span.RecordError(appErr)
		span.SetStatus(codes.Error, string(appErr.Code))
		return appErr
	}
	return nil
}

func rateLimitKey(messenger, userID string) string {
	return "ratelimit:" + messenger + ":" + userID
}
