// Пакет infralog предоставляет единый паттерн логирования, трейсинга и метрик
// для инфраструктурных компонентов (Redis, S3 и др.).

// start := time.Now()
// ctx, span := tracer.Start(ctx, "cache.Get")
// defer span.End()
// defer func() { met.RecordDuration(ctx, "cache", "Get", time.Since(start).Seconds()) }()
//
//	if err != nil {
//	    appErr := apperrors.Redis("cache.Get", err)
//	    infralog.Err(ctx, log, span, met, "cache", "Get", "не удалось получить значение", appErr, zap.String("key", key))
//	    return nil, appErr
//	}
package infralog

import (
	"context"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.uber.org/zap"

	apperrors "github.com/jsolteam/tenex-platform/internal/platform/errors"
	contextlog "github.com/jsolteam/tenex-platform/internal/platform/logger/context"
	"github.com/jsolteam/tenex-platform/internal/platform/logger/core"
	"github.com/jsolteam/tenex-platform/internal/platform/observability/metrics"
	"github.com/jsolteam/tenex-platform/internal/platform/observability/tracing"
)

// InfraMetrics — инструменты замера latency и ошибок для инфраструктурных компонентов.
type InfraMetrics struct {
	// OpDuration замеряет время выполнения операции в секундах.
	OpDuration metrics.Histogram

	// OpErrors считает ошибки с разбивкой по компоненту, методу и коду ошибки.
	OpErrors metrics.Counter
}

// NewInfraMetrics регистрирует гистограмму и счётчик в переданном реестре.
//
// Пример для Redis:
//
//	met, err := infralog.NewInfraMetrics(reg,
//	    "tenex_redis_op_duration_seconds",
//	    "Latency of Redis operations, in seconds.",
//	    "tenex_redis_errors_total",
//	    "Total number of Redis errors, by component, method and error code.",
//	)
func NewInfraMetrics(
	reg metrics.Registry,
	durationName, durationHelp string,
	errorsName, errorsHelp string,
) (*InfraMetrics, error) {
	dur, err := reg.Histogram(
		durationName,
		durationHelp,
		[]float64{0.0001, 0.0005, 0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1.0},
		LabelComponent, LabelMethod,
	)
	if err != nil {
		return nil, err
	}

	errs, err := reg.Counter(
		errorsName,
		errorsHelp,
		LabelComponent, LabelMethod, LabelCode,
	)
	if err != nil {
		return nil, err
	}

	return &InfraMetrics{OpDuration: dur, OpErrors: errs}, nil
}

// RecordDuration записывает latency операции в секундах.
// Вызывается через defer в начале каждого метода инфра-компонента:
//
//	start := time.Now()
//	defer met.RecordDuration(ctx, "cache", "Get", time.Since(start).Seconds())
func (m *InfraMetrics) RecordDuration(ctx context.Context, component, method string, elapsedSec float64) {
	m.OpDuration.Record(ctx, elapsedSec,
		metrics.L(LabelComponent, component),
		metrics.L(LabelMethod, method),
	)
}

// RecordError увеличивает счётчик ошибок для данного компонента, метода и кода.
//
//	met.RecordError(ctx, "cache", "Get", string(appErr.Code))
func (m *InfraMetrics) RecordError(ctx context.Context, component, method, code string) {
	m.OpErrors.Inc(ctx,
		metrics.L(LabelComponent, component),
		metrics.L(LabelMethod, method),
		metrics.L(LabelCode, code),
	)
}

// Стандартные лейблы для инфра-метрик.
const (
	LabelComponent = "component"
	LabelMethod    = "method"
	LabelCode      = "code"
)

// Err — единая точка обработки ошибок в инфра-компонентах.
// Выполняет три действия атомарно:
//  1. Записывает ошибку и статус в OpenTelemetry спан.
//  2. Инкрементирует счётчик ошибок в метриках.
//  3. Логирует сообщение на уровне, соответствующем severity ошибки.
func Err(
	ctx context.Context,
	l *core.Logger,
	span tracing.Span,
	met *InfraMetrics,
	component, method string,
	msg string,
	appErr *apperrors.AppError,
	extra ...zap.Field,
) {
	// ── Спан ──────────────────────────────────────────────────────────────
	span.RecordError(appErr)
	span.SetStatus(codes.Error, string(appErr.Code))
	span.SetAttributes(
		attribute.String("error.code", string(appErr.Code)),
		attribute.String("error.module", appErr.Module),
	)

	// ── Метрики ───────────────────────────────────────────────────────────
	met.RecordError(ctx, component, method, string(appErr.Code))

	// ── Лог ───────────────────────────────────────────────────────────────
	logFields := buildFields(appErr, extra...)
	cl := contextlog.FromCtx(ctx, l)

	switch appErr.Severity {
	case apperrors.SeverityFatal, apperrors.SeverityError:
		cl.Error(msg, logFields...)
	case apperrors.SeverityWarn:
		cl.Warn(msg, logFields...)
	case apperrors.SeverityInfo:
		cl.Info(msg, logFields...)
	default:
		cl.Debug(msg, logFields...)
	}
}

// Debug логирует штатные miss/noop ситуации на уровне Debug.
// Спан остаётся успешным, метрики ошибок не трогаются.
// Используется когда операция завершилась штатно, но результата нет
// (например, ключ не найден в кэше).
func Debug(ctx context.Context, l *core.Logger, msg string, extra ...zap.Field) {
	contextlog.FromCtx(ctx, l).Debug(msg, extra...)
}

// buildFields собирает []zap.Field: zap.Error + атрибуты AppError + extra.
func buildFields(appErr *apperrors.AppError, extra ...zap.Field) []zap.Field {
	out := make([]zap.Field, 0, 1+len(appErr.Attrs())+len(extra))
	out = append(out, zap.Error(appErr))
	for _, a := range appErr.Attrs() {
		out = append(out, zap.Any(a.Key, a.Val))
	}
	return append(out, extra...)
}
