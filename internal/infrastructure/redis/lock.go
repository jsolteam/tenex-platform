package redis

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"time"

	goredis "github.com/redis/go-redis/v9"
	"go.opentelemetry.io/otel/attribute"
	"go.uber.org/zap"

	"github.com/jsolteam/tenex-platform/internal/infrastructure/infralog"
	apperrors "github.com/jsolteam/tenex-platform/internal/platform/errors"
	"github.com/jsolteam/tenex-platform/internal/platform/observability/metrics"
)

// ErrLockNotAcquired возвращается если лок уже удерживается другим процессом.
var ErrLockNotAcquired = errors.New("lock: not acquired")

// Lock — хэндл удерживаемого распределённого лока.
// Получается через DistributedLocker.Acquire и освобождается через Release.
type Lock struct {
	client *Client
	met    *metrics.RedisMetrics
	key    string
	token  string // случайный токен для защиты от чужого Release
}

// Release освобождает лок.
// Безопасен для повторного вызова — второй вызов вернёт nil.
// Использует Lua-скрипт для атомарного сравнения токена перед удалением.
func (l *Lock) Release(ctx context.Context) error {
	ctx, span := l.client.tracer.Start(ctx, "lock.Release")
	defer span.End()
	start := time.Now()
	defer func() {
		l.met.RecordDuration(ctx, metrics.RedisComponentLock, "Release", time.Since(start).Seconds())
	}()
	span.SetAttributes(attribute.String("lock.key", l.key))

	const script = `
		if redis.call("GET", KEYS[1]) == ARGV[1] then
			return redis.call("DEL", KEYS[1])
		end
		return 0
	`

	result, err := l.client.rdb.Eval(ctx, script, []string{l.key}, l.token).Int()
	if err != nil && !errors.Is(err, goredis.Nil) {
		appErr := apperrors.Redis("lock.Release", err)
		infralog.Err(ctx, l.client.log, span, l.met,
			metrics.RedisComponentLock, "Release", "lock release failed", appErr,
			zap.String("key", l.key),
		)
		return appErr
	}

	if result == 0 {
		infralog.Debug(ctx, l.client.log, "lock already released or owned by another",
			zap.String("key", l.key),
		)
	}
	return nil
}

// DistributedLocker — создаёт распределённые локи через Redis SET NX EX.
//
// Назначение: предотвратить дублирование обработки одного reminder
// несколькими воркерами при горизонтальном масштабировании.
//
// Ключ: lock:{namespace}:{id}
// TTL:  передаётся при Acquire, защищает от вечного лока при падении воркера
type DistributedLocker struct {
	client *Client
	met    *metrics.RedisMetrics
}

// NewDistributedLocker создаёт DistributedLocker поверх существующего Client.
func NewDistributedLocker(c *Client) *DistributedLocker {
	return &DistributedLocker{client: c, met: c.met}
}

// Acquire пытается захватить лок.
//
//   - namespace: логическое пространство имён (например "reminder")
//   - id: уникальный идентификатор (например reminderID)
//   - ttl: время жизни лока (защита от зависшего воркера)
//
// Возвращает ErrLockNotAcquired если лок уже удерживается.
func (dl *DistributedLocker) Acquire(ctx context.Context, namespace, id string, ttl time.Duration) (*Lock, error) {
	ctx, span := dl.client.tracer.Start(ctx, "lock.Acquire")
	defer span.End()
	start := time.Now()
	defer func() {
		dl.met.RecordDuration(ctx, metrics.RedisComponentLock, "Acquire", time.Since(start).Seconds())
	}()

	key := lockKey(namespace, id)
	span.SetAttributes(
		attribute.String("lock.key", key),
		attribute.String("lock.ttl", ttl.String()),
	)

	token, err := randomToken()
	if err != nil {
		appErr := apperrors.Internal("lock.Acquire.token", err)
		return nil, appErr
	}

	ok, err := dl.client.rdb.SetNX(ctx, key, token, ttl).Result()
	if err != nil {
		appErr := apperrors.Redis("lock.Acquire", err)
		infralog.Err(ctx, dl.client.log, span, dl.met,
			metrics.RedisComponentLock, "Acquire", "lock acquire failed", appErr,
			zap.String("key", key),
		)
		return nil, appErr
	}

	if !ok {
		span.SetAttributes(attribute.Bool("lock.acquired", false))
		return nil, ErrLockNotAcquired
	}

	span.SetAttributes(attribute.Bool("lock.acquired", true))
	infralog.Debug(ctx, dl.client.log, "lock acquired",
		zap.String("key", key),
		zap.Duration("ttl", ttl),
	)
	return &Lock{client: dl.client, met: dl.met, key: key, token: token}, nil
}

// lockKey: lock:{namespace}:{id}
func lockKey(namespace, id string) string {
	return "lock:" + namespace + ":" + id
}

func randomToken() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
