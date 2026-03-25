package components

import (
	"context"
	"fmt"

	infraredis "github.com/jsolteam/tenex-platform/internal/infrastructure/redis"
	"github.com/jsolteam/tenex-platform/internal/platform/logger/facade"
	"github.com/jsolteam/tenex-platform/internal/platform/observability/metrics"
	"github.com/jsolteam/tenex-platform/internal/platform/observability/tracing"
)

type RedisComponent struct {
	cfg    *ConfigComponent
	tracer *TracingComponent
	met    *MetricsComponent

	client         *infraredis.Client
	FSMStore       *infraredis.FSMStore
	SchedulerQueue *infraredis.SchedulerQueue
	Cache          *infraredis.Cache
	RateLimiter    *infraredis.RateLimiter
	Locker         *infraredis.DistributedLocker
}

// NewRedis создаёт RedisComponent.
// cfg — обязателен. tracer — может быть nil (будет использован noop).
func NewRedis(cfg *ConfigComponent, tracer *TracingComponent, met *MetricsComponent) *RedisComponent {
	return &RedisComponent{cfg: cfg, tracer: tracer, met: met}
}

// Start подключается к Redis и инициализирует все компоненты.
func (r *RedisComponent) Start(ctx context.Context) error {
	appCfg := r.cfg.Get()

	// Logger берётся из глобального facade — он уже инициализирован
	// к моменту старта RedisComponent (logger стартует раньше в контейнере).
	log := facade.L()

	var tr tracing.Tracer
	if r.tracer != nil {
		tr = r.tracer.Tracer()
	} else {
		tr = tracing.NewNoop()
	}

	redisCfg := infraredis.Config{
		Addr:     appCfg.Redis.Addr,
		Password: appCfg.Redis.Password,
		DB:       appCfg.Redis.DB,
	}

	var (
		redisMet *metrics.RedisMetrics
		err      error
	)
	if r.met != nil {
		redisMet, err = metrics.NewRedisMetrics(r.met.Registry())
		if err != nil {
			return fmt.Errorf("redis component metrics: %w", err)
		}
	} else {
		redisMet, _ = metrics.NewRedisMetrics(metrics.NewNoop())
	}

	client, err := infraredis.New(ctx, redisCfg, log, tr, redisMet)
	if err != nil {
		return fmt.Errorf("redis component: %w", err)
	}

	r.client = client
	r.FSMStore = infraredis.NewFSMStore(client)
	r.SchedulerQueue = infraredis.NewSchedulerQueue(client)
	r.Cache = infraredis.NewCache(client)
	r.RateLimiter = infraredis.NewRateLimiter(client)
	r.Locker = infraredis.NewDistributedLocker(client)

	return nil
}

// Stop закрывает соединение с Redis.
func (r *RedisComponent) Stop(_ context.Context) error {
	if r.client != nil {
		return r.client.Close()
	}
	return nil
}

// Client возвращает нижележащий Redis-клиент.
// Паникует если вызван до Start().
func (r *RedisComponent) Client() *infraredis.Client {
	if r.client == nil {
		panic("RedisComponent.Client() called before Start()")
	}
	return r.client
}
