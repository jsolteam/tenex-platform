package redis

import (
	"context"
	"time"

	goredis "github.com/redis/go-redis/v9"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.uber.org/zap"

	apperrors "github.com/jsolteam/tenex-platform/internal/platform/errors"
	contextlog "github.com/jsolteam/tenex-platform/internal/platform/logger/context"
	"github.com/jsolteam/tenex-platform/internal/platform/logger/core"
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
}

// New создаёт нового клиента, проверяет соединение через Ping и возвращает готовый Client.
// Возвращает apperrors.ErrRedisUnavailable при любой ошибке подключения.
func New(ctx context.Context, cfg Config, log *core.Logger, tracer tracing.Tracer) (*Client, error) {
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

	if err := rdb.Ping(ctx2).Err(); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, string(apperrors.ErrRedisUnavailable))
		span.SetAttributes(attribute.String("redis.addr", cfg.Addr))

		appErr := apperrors.Redis("redis.New.ping", err)
		contextlog.FromCtx(ctx, l).Error("redis ping failed",
			zap.Error(appErr),
			zap.String("addr", cfg.Addr),
		)
		_ = rdb.Close()
		return nil, appErr
	}

	contextlog.FromCtx(ctx, l).Info("redis connected", zap.String("addr", cfg.Addr))
	return &Client{rdb: rdb, log: l, tracer: tracer}, nil
}

// Close закрывает соединение с Redis.
// Логирует ошибку при закрытии, но всегда возвращает её наружу.
func (c *Client) Close() error {
	if err := c.rdb.Close(); err != nil {
		appErr := apperrors.Redis("redis.Close", err)
		c.log.Error("redis close failed", zap.Error(appErr))
		return appErr
	}
	c.log.Info("redis connection closed")
	return nil
}

// Ping проверяет доступность Redis.
// Используется в health-checks.
func (c *Client) Ping(ctx context.Context) error {
	ctx, span := c.tracer.Start(ctx, "redis.Ping")
	defer span.End()

	if err := c.rdb.Ping(ctx).Err(); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, string(apperrors.ErrRedisUnavailable))

		appErr := apperrors.Redis("redis.Ping", err).Retryable()
		contextlog.FromCtx(ctx, c.log).Error("redis ping failed", zap.Error(appErr))
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
