package infralog_unit

import (
	"context"
	"errors"
	"testing"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"

	"github.com/jsolteam/tenex-platform/internal/infrastructure/infralog"
	apperrors "github.com/jsolteam/tenex-platform/internal/platform/errors"
	logcore "github.com/jsolteam/tenex-platform/internal/platform/logger/core"
	"github.com/jsolteam/tenex-platform/internal/platform/observability/metrics"
	"github.com/jsolteam/tenex-platform/internal/platform/observability/tracing"
)

// newObservedLogger возвращает логгер, записывающий все сообщения в память,
// и функцию доступа к ним — удобно для проверки уровня и содержимого логов.
func newObservedLogger(t *testing.T) (*logcore.Logger, *observer.ObservedLogs) {
	t.Helper()
	zc, logs := observer.New(zapcore.DebugLevel)
	return logcore.New(zap.New(zc)), logs
}

// newNoopMetrics возвращает *infralog.InfraMetrics с noop-реестром.
func newNoopMetrics(t *testing.T) *infralog.InfraMetrics {
	t.Helper()
	reg := metrics.NewNoop()
	met, err := infralog.NewInfraMetrics(reg,
		"test_op_duration_seconds", "тест latency",
		"test_op_errors_total", "тест ошибок",
	)
	if err != nil {
		t.Fatalf("NewInfraMetrics: %v", err)
	}
	return met
}

// TestErr_LogsAtErrorLevel проверяет что ошибка с SeverityError логируется на уровне Error.
func TestErr_LogsAtErrorLevel(t *testing.T) {
	l, logs := newObservedLogger(t)
	met := newNoopMetrics(t)
	span := tracing.NewNoop()
	_, s := span.Start(context.Background(), "test")

	appErr := apperrors.Redis("cache.Get", errors.New("connection refused"))
	infralog.Err(context.Background(), l, s, met, "cache", "Get", "не удалось получить ключ", appErr)

	if logs.Len() != 1 {
		t.Fatalf("ожидали 1 лог-запись, получили %d", logs.Len())
	}
	entry := logs.All()[0]
	if entry.Level != zapcore.ErrorLevel {
		t.Errorf("уровень лога = %v, ожидали Error", entry.Level)
	}
	if entry.Message != "не удалось получить ключ" {
		t.Errorf("сообщение = %q, ожидали 'не удалось получить ключ'", entry.Message)
	}
}

// TestErr_LogsAtWarnLevel проверяет что retryable-ошибка логируется на уровне Warn.
func TestErr_LogsAtWarnLevel(t *testing.T) {
	l, logs := newObservedLogger(t)
	met := newNoopMetrics(t)
	_, s := tracing.NewNoop().Start(context.Background(), "test")

	appErr := apperrors.Redis("cache.Get", errors.New("timeout")).Retryable()
	infralog.Err(context.Background(), l, s, met, "cache", "Get", "таймаут Redis", appErr)

	if logs.Len() != 1 {
		t.Fatalf("ожидали 1 лог-запись, получили %d", logs.Len())
	}
	if logs.All()[0].Level != zapcore.WarnLevel {
		t.Errorf("уровень лога = %v, ожидали Warn", logs.All()[0].Level)
	}
}

// TestErr_IncludesExtraFields проверяет что дополнительные zap.Field попадают в лог.
func TestErr_IncludesExtraFields(t *testing.T) {
	l, logs := newObservedLogger(t)
	met := newNoopMetrics(t)
	_, s := tracing.NewNoop().Start(context.Background(), "test")

	appErr := apperrors.S3("s3.Upload", errors.New("no such bucket"))
	infralog.Err(context.Background(), l, s, met, "s3", "Upload", "ошибка загрузки", appErr,
		zap.String("key", "media/photo.jpg"),
		zap.Int64("size", 1024),
	)

	entry := logs.All()[0]
	keys := map[string]bool{}
	for _, f := range entry.Context {
		keys[f.Key] = true
	}
	for _, want := range []string{"key", "size"} {
		if !keys[want] {
			t.Errorf("поле %q отсутствует в лог-записи", want)
		}
	}
}

// TestErr_AttrsFromAppErrorInLog проверяет что атрибуты AppError.With() попадают в лог.
func TestErr_AttrsFromAppErrorInLog(t *testing.T) {
	l, logs := newObservedLogger(t)
	met := newNoopMetrics(t)
	_, s := tracing.NewNoop().Start(context.Background(), "test")

	appErr := apperrors.Redis("lock.Acquire", errors.New("key exists")).
		With("namespace", "reminder").
		With("id", "42")
	infralog.Err(context.Background(), l, s, met, "lock", "Acquire", "не удалось захватить лок", appErr)

	entry := logs.All()[0]
	keys := map[string]bool{}
	for _, f := range entry.Context {
		keys[f.Key] = true
	}
	for _, want := range []string{"namespace", "id"} {
		if !keys[want] {
			t.Errorf("атрибут AppError %q отсутствует в лог-записи", want)
		}
	}
}

// TestDebug_LogsAtDebugLevel проверяет что Debug пишет на уровне Debug без ошибки.
func TestDebug_LogsAtDebugLevel(t *testing.T) {
	l, logs := newObservedLogger(t)

	infralog.Debug(context.Background(), l, "ключ не найден в кэше", zap.String("key", "fsm:tg:123"))

	if logs.Len() != 1 {
		t.Fatalf("ожидали 1 лог-запись, получили %d", logs.Len())
	}
	entry := logs.All()[0]
	if entry.Level != zapcore.DebugLevel {
		t.Errorf("уровень лога = %v, ожидали Debug", entry.Level)
	}
	if entry.Message != "ключ не найден в кэше" {
		t.Errorf("сообщение = %q", entry.Message)
	}
}

// TestDebug_NoMetricsOrSpanSideEffects — Debug не трогает метрики и спан,
// компилируется без met и span в сигнатуре.
func TestDebug_NoMetricsOrSpanSideEffects(t *testing.T) {
	l, logs := newObservedLogger(t)

	// Намеренно не создаём met и span — Debug их не принимает.
	infralog.Debug(context.Background(), l, "cache miss")

	if logs.Len() != 1 {
		t.Fatalf("ожидали 1 лог-запись, получили %d", logs.Len())
	}
}

// TestNewInfraMetrics_Prometheus проверяет создание InfraMetrics с реальным Prometheus-реестром.
func TestNewInfraMetrics_Prometheus(t *testing.T) {
	reg := metrics.NewPrometheus()
	met, err := infralog.NewInfraMetrics(reg,
		"tenex_redis_op_duration_seconds", "Latency of Redis operations, in seconds.",
		"tenex_redis_errors_total", "Total number of Redis errors.",
	)
	if err != nil {
		t.Fatalf("NewInfraMetrics: %v", err)
	}
	if met == nil {
		t.Fatal("NewInfraMetrics вернул nil")
	}
}

// TestNewInfraMetrics_IdempotentRegistration проверяет что повторная регистрация
// одних и тех же метрик не возвращает ошибку (аналогично DBMetrics).
func TestNewInfraMetrics_IdempotentRegistration(t *testing.T) {
	reg := metrics.NewPrometheus()
	_, err := infralog.NewInfraMetrics(reg,
		"tenex_s3_op_duration_seconds", "S3 latency.",
		"tenex_s3_errors_total", "S3 errors.",
	)
	if err != nil {
		t.Fatalf("первый NewInfraMetrics: %v", err)
	}
	_, err = infralog.NewInfraMetrics(reg,
		"tenex_s3_op_duration_seconds", "S3 latency.",
		"tenex_s3_errors_total", "S3 errors.",
	)
	if err != nil {
		t.Fatalf("повторный NewInfraMetrics (дубликат) не должен ошибаться: %v", err)
	}
}

// TestInfraMetrics_RecordDurationNoPanic проверяет что RecordDuration не паникует.
func TestInfraMetrics_RecordDurationNoPanic(t *testing.T) {
	met := newNoopMetrics(t)
	met.RecordDuration(context.Background(), "cache", "Get", 0.001)
	met.RecordDuration(context.Background(), "fsm", "Set", 0.0)
}

// TestInfraMetrics_RecordErrorNoPanic проверяет что RecordError не паникует.
func TestInfraMetrics_RecordErrorNoPanic(t *testing.T) {
	met := newNoopMetrics(t)
	met.RecordError(context.Background(), "cache", "Get", "REDIS_UNAVAILABLE")
	met.RecordError(context.Background(), "s3", "Upload", "S3_ERROR")
}
