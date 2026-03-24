package metrics_unit

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jsolteam/tenex-platform/internal/platform/observability/metrics"
)

// TestNewRedisMetrics_ReturnsNonNil проверяет что конструктор не возвращает nil.
func TestNewRedisMetrics_ReturnsNonNil(t *testing.T) {
	reg := metrics.NewPrometheus()
	met, err := metrics.NewRedisMetrics(reg)
	if err != nil {
		t.Fatalf("NewRedisMetrics: %v", err)
	}
	if met == nil {
		t.Fatal("NewRedisMetrics вернул nil")
	}
}

// TestNewRedisMetrics_WithNoop проверяет работу с noop-реестром без паник.
func TestNewRedisMetrics_WithNoop(t *testing.T) {
	reg := metrics.NewNoop()
	met, err := metrics.NewRedisMetrics(reg)
	if err != nil {
		t.Fatalf("NewRedisMetrics(noop): %v", err)
	}
	if met == nil {
		t.Fatal("NewRedisMetrics(noop) вернул nil")
	}
}

// TestNewRedisMetrics_IdempotentRegistration проверяет что повторная регистрация
// тех же метрик не возвращает ошибку.
func TestNewRedisMetrics_IdempotentRegistration(t *testing.T) {
	reg := metrics.NewPrometheus()
	if _, err := metrics.NewRedisMetrics(reg); err != nil {
		t.Fatalf("первый NewRedisMetrics: %v", err)
	}
	if _, err := metrics.NewRedisMetrics(reg); err != nil {
		t.Fatalf("повторный NewRedisMetrics не должен ошибаться: %v", err)
	}
}

// TestRedisMetrics_RecordDuration_NoPanic проверяет вызовы RecordDuration
// для всех стандартных компонентов.
func TestRedisMetrics_RecordDuration_NoPanic(t *testing.T) {
	reg := metrics.NewPrometheus()
	met, _ := metrics.NewRedisMetrics(reg)
	ctx := context.Background()

	cases := []struct {
		component string
		method    string
		elapsed   float64
	}{
		{metrics.RedisComponentCache, "Get", 0.001},
		{metrics.RedisComponentCache, "Set", 0.002},
		{metrics.RedisComponentCache, "Delete", 0.0005},
		{metrics.RedisComponentCache, "Exists", 0.0003},
		{metrics.RedisComponentFSM, "Get", 0.001},
		{metrics.RedisComponentFSM, "Set", 0.002},
		{metrics.RedisComponentFSM, "Delete", 0.0005},
		{metrics.RedisComponentQueue, "Enqueue", 0.001},
		{metrics.RedisComponentQueue, "PollDue", 0.005},
		{metrics.RedisComponentQueue, "Remove", 0.0008},
		{metrics.RedisComponentQueue, "Size", 0.0002},
		{metrics.RedisComponentLock, "Acquire", 0.002},
		{metrics.RedisComponentLock, "Release", 0.001},
		{metrics.RedisComponentRateLimiter, "Allow", 0.003},
		{metrics.RedisComponentRateLimiter, "Reset", 0.001},
	}

	for _, c := range cases {
		met.RecordDuration(ctx, c.component, c.method, c.elapsed)
	}
}

// TestRedisMetrics_RecordError_NoPanic проверяет вызовы RecordError
// для разных кодов ошибок.
func TestRedisMetrics_RecordError_NoPanic(t *testing.T) {
	reg := metrics.NewPrometheus()
	met, _ := metrics.NewRedisMetrics(reg)
	ctx := context.Background()

	met.RecordError(ctx, metrics.RedisComponentCache, "Get", "REDIS_UNAVAILABLE")
	met.RecordError(ctx, metrics.RedisComponentFSM, "Set", "REDIS_UNAVAILABLE")
	met.RecordError(ctx, metrics.RedisComponentQueue, "Enqueue", "REDIS_UNAVAILABLE")
	met.RecordError(ctx, metrics.RedisComponentLock, "Acquire", "INTERNAL")
	met.RecordError(ctx, metrics.RedisComponentRateLimiter, "Allow", "REDIS_UNAVAILABLE")
}

// TestRedisMetrics_AppearInMetricsEndpoint — ключевой тест задания:
// обе метрики должны появляться в /metrics после записи значений.
func TestRedisMetrics_AppearInMetricsEndpoint(t *testing.T) {
	reg := metrics.NewPrometheus()
	met, err := metrics.NewRedisMetrics(reg)
	if err != nil {
		t.Fatalf("NewRedisMetrics: %v", err)
	}

	ctx := context.Background()

	// Записываем по одному значению в каждую метрику.
	met.RecordDuration(ctx, metrics.RedisComponentCache, "Get", 0.001)
	met.RecordDuration(ctx, metrics.RedisComponentFSM, "Set", 0.002)
	met.RecordError(ctx, metrics.RedisComponentCache, "Get", "REDIS_UNAVAILABLE")
	met.RecordError(ctx, metrics.RedisComponentLock, "Acquire", "INTERNAL")

	// Снимаем срез через HTTP handler реестра.
	w := httptest.NewRecorder()
	reg.Handler().ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/metrics", nil))

	if w.Code != http.StatusOK {
		t.Fatalf("статус /metrics = %d, ожидали 200", w.Code)
	}

	body, _ := io.ReadAll(w.Result().Body)
	bodyStr := string(body)

	// Проверяем наличие обеих метрик по имени.
	wantMetrics := []string{
		"tenex_redis_op_duration_seconds",
		"tenex_redis_errors_total",
	}
	for _, want := range wantMetrics {
		if !strings.Contains(bodyStr, want) {
			t.Errorf("метрика %q отсутствует в /metrics output", want)
		}
	}

	// Проверяем наличие конкретных лейблов в выводе.
	wantLabels := []string{
		`component="cache"`,
		`component="fsm"`,
		`component="lock"`,
		`method="Get"`,
		`method="Set"`,
		`method="Acquire"`,
		`code="REDIS_UNAVAILABLE"`,
	}
	for _, want := range wantLabels {
		if !strings.Contains(bodyStr, want) {
			t.Errorf("лейбл %q отсутствует в /metrics output", want)
		}
	}
}

// TestRedisMetrics_DurationBucketsPresent проверяет что гистограмма имеет бакеты —
// в выводе должны быть строки с суффиксом _bucket.
func TestRedisMetrics_DurationBucketsPresent(t *testing.T) {
	reg := metrics.NewPrometheus()
	met, _ := metrics.NewRedisMetrics(reg)

	met.RecordDuration(context.Background(), metrics.RedisComponentQueue, "PollDue", 0.003)

	w := httptest.NewRecorder()
	reg.Handler().ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/metrics", nil))

	body, _ := io.ReadAll(w.Result().Body)
	if !strings.Contains(string(body), "tenex_redis_op_duration_seconds_bucket") {
		t.Error("бакеты гистограммы отсутствуют в /metrics output")
	}
}

// TestRedisComponentConstants проверяет что все константы компонентов
// не пустые и уникальные.
func TestRedisComponentConstants(t *testing.T) {
	components := []string{
		metrics.RedisComponentCache,
		metrics.RedisComponentFSM,
		metrics.RedisComponentQueue,
		metrics.RedisComponentLock,
		metrics.RedisComponentRateLimiter,
	}

	seen := map[string]bool{}
	for _, c := range components {
		if c == "" {
			t.Error("константа компонента не должна быть пустой строкой")
		}
		if seen[c] {
			t.Errorf("дублирующаяся константа компонента: %q", c)
		}
		seen[c] = true
	}
}
