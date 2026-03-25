package redis

import (
	"context"
	"time"

	"github.com/jsolteam/tenex-platform/internal/infrastructure/infralog"
	goredis "github.com/redis/go-redis/v9"
	"go.uber.org/zap"

	apperrors "github.com/jsolteam/tenex-platform/internal/platform/errors"
	"github.com/jsolteam/tenex-platform/internal/platform/logger/core"
	"github.com/jsolteam/tenex-platform/internal/platform/observability/metrics"
	"github.com/jsolteam/tenex-platform/internal/platform/observability/tracing"
)

const (
	defaultDialTimeout  = 5 * time.Second
	defaultReadTimeout  = 3 * time.Second
	defaultWriteTimeout = 3 * time.Second
	defaultPoolSize     = 10
	defaultMinIdleConns = 2
	pingTimeout         = 5 * time.Second
)

type Config struct {
	Addr     string
	Password string
	DB       int
}

type Client struct {
	rdb    *goredis.Client
	log    *core.Logger
	tracer tracing.Tracer
	met    *metrics.RedisMetrics
}

func (c *Client) redisMetrics() *metrics.RedisMetrics {
	if c.met == nil {
		noopMet, _ := metrics.NewRedisMetrics(metrics.NewNoop())
		c.met = noopMet
	}
	return c.met
}

// New создаёт нового клиента, проверяет соединение через Ping и возвращает готовый Client.
// Возвращает apperrors.ErrRedisUnavailable при любой ошибке подключения.
func New(
	ctx context.Context,
	cfg Config,
	log *core.Logger,
	tracer tracing.Tracer,
	metOpt ...*metrics.RedisMetrics,
) (*Client, error) {
	var met *metrics.RedisMetrics
	if len(metOpt) > 0 {
		met = metOpt[0]
	}
	if met == nil {
		noopMet, _ := metrics.NewRedisMetrics(metrics.NewNoop())
		met = noopMet
	}

	l := log.With(zap.String("component", "redis"), zap.String("addr", cfg.Addr))

	rdb := goredis.NewClient(&goredis.Options{
		Addr:         cfg.Addr,
		Password:     cfg.Password,
		DB:           cfg.DB,
		DialTimeout:  defaultDialTimeout,
		ReadTimeout:  defaultReadTimeout,
		WriteTimeout: defaultWriteTimeout,
		PoolSize:     defaultPoolSize,
		MinIdleConns: defaultMinIdleConns,
	})

	pingCtx, cancel := context.WithTimeout(ctx, pingTimeout)
	defer cancel()

	ctx2, span := tracer.Start(pingCtx, "redis.New.ping")
	defer span.End()
	start := time.Now()
	defer func() {
		met.RecordDuration(ctx2, "client", "New.ping", time.Since(start).Seconds())
	}()

	if err := rdb.Ping(ctx2).Err(); err != nil {
		appErr := apperrors.Redis("redis.New.ping", err)
		infralog.Err(ctx2, l, span, met,
			"client", "New.ping", "redis ping failed", appErr,
			zap.String("addr", cfg.Addr),
		)
		_ = rdb.Close()
		return nil, appErr
	}
	return &Client{rdb: rdb, log: l, tracer: tracer, met: met}, nil
}

// Close закрывает соединение с Redis.
// Логирует ошибку при закрытии, но всегда возвращает её наружу.
func (c *Client) Close() error {
	start := time.Now()
	defer func() {
		c.redisMetrics().RecordDuration(context.Background(), "client", "Close", time.Since(start).Seconds())
	}()

	if err := c.rdb.Close(); err != nil {
		appErr := apperrors.Redis("redis.Close", err)
		ctx, span := c.tracer.Start(context.Background(), "redis.Close")
		defer span.End()
		infralog.Err(ctx, c.log, span, c.redisMetrics(), "client", "Close", "redis close failed", appErr)
		return appErr
	}
	return nil
}

// Ping проверяет доступность Redis.
// Используется в health-checks.
func (c *Client) Ping(ctx context.Context) error {
	ctx, span := c.tracer.Start(ctx, "redis.Ping")
	defer span.End()
	start := time.Now()
	defer func() {
		c.redisMetrics().RecordDuration(ctx, "client", "Ping", time.Since(start).Seconds())
	}()

	if err := c.rdb.Ping(ctx).Err(); err != nil {
		appErr := apperrors.Redis("redis.Ping", err).Retryable()
		infralog.Err(ctx, c.log, span, c.redisMetrics(), "client", "Ping", "redis ping failed", appErr)
		return appErr
	}
	return nil
}

// Underlying возвращает нижележащий *goredis.Client для прямого использования
// в FSM Store, Scheduler Queue и других инфраструктурных компонентах.
// Не использовать в прикладном коде — только в инфраструктурном слое.
func (c *Client) Underlying() *goredis.Client {
	return c.rdb
}
