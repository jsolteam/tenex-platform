package metrics

import "context"

// RedisMetrics — инструменты замера latency и ошибок для всех Redis-операций платформы.
//
// Метрики:
//
//	tenex_redis_op_duration_seconds{component, method}  — latency каждой операции
//	tenex_redis_errors_total{component, method, code}   — счётчик ошибок по коду
type RedisMetrics struct {
	OpDuration Histogram
	OpErrors   Counter
}

const (
	RedisComponentCache       = "cache"
	RedisComponentFSM         = "fsm"
	RedisComponentQueue       = "queue"
	RedisComponentLock        = "lock"
	RedisComponentRateLimiter = "ratelimit"
)

var redisDurationBuckets = []float64{
	0.0001, 0.0005, 0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1.0,
}

func NewRedisMetrics(reg Registry) (*RedisMetrics, error) {
	dur, err := reg.Histogram(
		"tenex_redis_op_duration_seconds",
		"Latency of Redis operations, in seconds.",
		redisDurationBuckets,
		LabelComponent, LabelMethod,
	)
	if err != nil {
		return nil, err
	}

	errs, err := reg.Counter(
		"tenex_redis_errors_total",
		"Total number of Redis errors, by component, method and error code.",
		LabelComponent, LabelMethod, LabelCode,
	)
	if err != nil {
		return nil, err
	}

	return &RedisMetrics{OpDuration: dur, OpErrors: errs}, nil
}

func (m *RedisMetrics) RecordDuration(ctx context.Context, component, method string, elapsedSec float64) {
	m.OpDuration.Record(ctx, elapsedSec,
		L(LabelComponent, component),
		L(LabelMethod, method),
	)
}

func (m *RedisMetrics) RecordError(ctx context.Context, component, method, code string) {
	m.OpErrors.Inc(ctx,
		L(LabelComponent, component),
		L(LabelMethod, method),
		L(LabelCode, code),
	)
}
