package e2e

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"go.uber.org/zap"

	"github.com/jsol/tenex-platform/internal/platform/config"
	"github.com/jsol/tenex-platform/internal/platform/logger"
	"github.com/jsol/tenex-platform/internal/platform/logger/facade"
)

func configYAML(env, logLevel string) []byte {
	return []byte(`
app:
  env: ` + env + `
  name: tenex-e2e
  log_level: ` + logLevel + `
db:
  host: localhost
  port: 5432
  user: tenex
  name: tenex
redis:
  addr: "localhost:6379"
scheduler:
  max_retries: 3
  reminder_retry_interval: 30s
`)
}

func writeCfg(t *testing.T, path string, content []byte) {
	t.Helper()
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatalf("writeCfg: %v", err)
	}
}

func waitFor(t *testing.T, timeout time.Duration, cond func() bool) bool {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if cond() {
			return true
		}
		time.Sleep(20 * time.Millisecond)
	}
	return false
}

// ── сценарий 1: базовый старт и остановка ─────────────────────────────────────

// TestE2E_PlatformStartsAndStops проверяет:
//  1. config.Bootstrap читает YAML-файл
//  2. logger.Bootstrap инициализирует логгер
//  3. facade.L() возвращает рабочий логгер
//  4. Graceful shutdown завершается без таймаута
func TestE2E_PlatformStartsAndStops(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	writeCfg(t, cfgPath, configYAML("local", "info"))

	mgr, cfgShutdown, err := config.Bootstrap(context.Background(), cfgPath)
	if err != nil {
		t.Fatalf("config.Bootstrap: %v", err)
	}

	logShutdown, err := logger.Bootstrap(mgr)
	if err != nil {
		cfgShutdown()
		t.Fatalf("logger.Bootstrap: %v", err)
	}

	// facade.L() должен работать после инициализации.
	l := facade.L()
	if l == nil {
		t.Fatal("facade.L() returned nil after bootstrap")
	}
	l.Info("e2e: platform started", zap.String("test", t.Name()))

	// Graceful shutdown.
	done := make(chan struct{})
	go func() {
		if err := logShutdown(); err != nil {
			t.Errorf("logShutdown: %v", err)
		}
		cfgShutdown()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("graceful shutdown timed out — goroutine leak or AsyncCore.Close() blocked")
	}
}

// ── сценарий 2: hot-reload конфига ───────────────────────────────────────────

// TestE2E_HotReloadConfigFile проверяет:
//  1. Изменение YAML-файла → watcher замечает → Manager.Load() вызывается
//  2. Listener получает уведомление с новыми данными
//  3. Платформа продолжает работу после reload'а
func TestE2E_HotReloadConfigFile(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	writeCfg(t, cfgPath, configYAML("local", "info"))

	mgr, cfgShutdown, err := config.Bootstrap(context.Background(), cfgPath)
	if err != nil {
		t.Fatalf("config.Bootstrap: %v", err)
	}
	defer cfgShutdown()

	logShutdown, err := logger.Bootstrap(mgr)
	if err != nil {
		t.Fatalf("logger.Bootstrap: %v", err)
	}
	defer logShutdown() //nolint:errcheck

	var reloads atomic.Int64
	mgr.AddListener(func(old, newCfg *config.AppConfig) {
		reloads.Add(1)
	})

	// Меняем файл — watcher должен поймать и выполнить reload.
	time.Sleep(50 * time.Millisecond)
	writeCfg(t, cfgPath, configYAML("staging", "debug"))

	if !waitFor(t, 3*time.Second, func() bool { return reloads.Load() >= 1 }) {
		t.Fatal("config hot-reload did not trigger after file change")
	}

	// После reload'а конфиг должен содержать новые значения.
	cfg := mgr.Get()
	if cfg.App.Env != "staging" {
		t.Errorf("after reload App.Env = %q, want %q", cfg.App.Env, "staging")
	}

	// Логгер должен продолжать работать.
	facade.L().Info("e2e: after hot-reload", zap.String("env", cfg.App.Env))
}

// ── сценарий 3: atomic write (vim/kubectl style) ──────────────────────────────

// TestE2E_AtomicFileReplaceRecovers проверяет:
//  1. Файл заменяется атомарно через rename (как это делают vim, kubectl, envsubst)
//  2. Watcher теряет inode → запускает rewatchWithBackoff
//  3. После recovery reload всё равно происходит
func TestE2E_AtomicFileReplaceRecovers(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	writeCfg(t, cfgPath, configYAML("local", "info"))

	mgr, cfgShutdown, err := config.Bootstrap(context.Background(), cfgPath)
	if err != nil {
		t.Fatalf("config.Bootstrap: %v", err)
	}
	defer cfgShutdown()

	logShutdown, err := logger.Bootstrap(mgr)
	if err != nil {
		t.Fatalf("logger.Bootstrap: %v", err)
	}
	defer logShutdown() //nolint:errcheck

	var reloads atomic.Int64
	mgr.AddListener(func(_, _ *config.AppConfig) { reloads.Add(1) })

	// Атомарная замена: write tmp → rename tmp → cfgPath.
	tmpPath := cfgPath + ".new"
	writeCfg(t, tmpPath, configYAML("production", "warn"))
	if err := os.Rename(tmpPath, cfgPath); err != nil {
		t.Fatalf("atomic rename: %v", err)
	}

	if !waitFor(t, 4*time.Second, func() bool { return reloads.Load() >= 1 }) {
		t.Fatal("watcher did not recover after atomic file replace (H-4 regression)")
	}

	cfg := mgr.Get()
	if cfg.App.Env != "production" {
		t.Errorf("after atomic replace App.Env = %q, want %q", cfg.App.Env, "production")
	}
}

// ── сценарий 4: логгер + Loki endpoint ───────────────────────────────────────

// TestE2E_LoggerWithLokiEndpoint проверяет:
//  1. config содержит loki_endpoint
//  2. logger.Bootstrap создаёт Loki writer и запускает Run()
//  3. Записи уходят в Loki (fake httptest server)
//  4. Shutdown корректно завершает Loki writer
//
// Стратегия: loki_endpoint передаётся через LOKI_ENDPOINT env var.
// t.Setenv гарантирует что EnvProvider (Required) передаёт значение через
// v.Set() с наивысшим приоритетом — в отличие от YAML deep merge через
// FileProvider, где viper.MergeConfigMap может не доставить значение
// для ключей без SetDefault при пустом начальном состоянии.
func TestE2E_LoggerWithLokiEndpoint(t *testing.T) {
	var lokiHits atomic.Int64
	lokiSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		lokiHits.Add(1)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer lokiSrv.Close()

	// Передаём loki endpoint через env — EnvProvider читает LOKI_ENDPOINT и
	// вызывает v.Set("observability.loki_endpoint", url) с override-приоритетом.
	t.Setenv("LOKI_ENDPOINT", lokiSrv.URL)

	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	writeCfg(t, cfgPath, configYAML("local", "info")) // базовый YAML без loki_endpoint

	mgr, cfgShutdown, err := config.Bootstrap(context.Background(), cfgPath)
	if err != nil {
		t.Fatalf("config.Bootstrap: %v", err)
	}
	defer cfgShutdown()

	// Проверяем что LokiEndpoint действительно дошёл до конфига.
	if got := mgr.Get().Observability.LokiEndpoint; got != lokiSrv.URL {
		t.Fatalf("LokiEndpoint = %q, want %q — env var not propagated", got, lokiSrv.URL)
	}

	logShutdown, err := logger.Bootstrap(mgr)
	if err != nil {
		t.Fatalf("logger.Bootstrap: %v", err)
	}

	// Пишем несколько сообщений.
	l := facade.L()
	for i := 0; i < 5; i++ {
		l.Info("e2e-loki-test", zap.Int("i", i))
	}

	// Graceful shutdown: AsyncCore.Close() дренирует очередь → LokiCore.Write() → batch.
	// stop() ждёт Run() → flush() → HTTP POST → <-runDone.
	if err := logShutdown(); err != nil {
		t.Errorf("logShutdown: %v", err)
	}

	// После shutdown Loki должен был получить хотя бы один запрос.
	if lokiHits.Load() == 0 {
		t.Error("Loki received 0 requests after graceful shutdown — flush missing")
	}
}

// ── сценарий 5: конкурентный старт нескольких экземпляров ────────────────────

// TestE2E_ConcurrentBootstraps проверяет что несколько одновременных Bootstrap
// не создают race conditions (например, при параллельных тестах сервисов).
func TestE2E_ConcurrentBootstraps(t *testing.T) {
	const n = 3
	errs := make(chan error, n)
	shutdowns := make(chan func() error, n)

	for i := 0; i < n; i++ {
		go func(i int) {
			dir := t.TempDir()
			cfgPath := filepath.Join(dir, "config.yaml")
			writeCfg(t, cfgPath, configYAML("local", "info"))

			mgr, cfgShutdown, err := config.Bootstrap(context.Background(), cfgPath)
			if err != nil {
				errs <- err
				return
			}

			logShutdown, err := logger.Bootstrap(mgr)
			if err != nil {
				cfgShutdown()
				errs <- err
				return
			}

			shutdowns <- func() error {
				err := logShutdown()
				cfgShutdown()
				return err
			}
			errs <- nil
		}(i)
	}

	for i := 0; i < n; i++ {
		if err := <-errs; err != nil {
			t.Errorf("bootstrap[%d] failed: %v", i, err)
		}
	}

	close(shutdowns)
	for sd := range shutdowns {
		done := make(chan error, 1)
		go func(s func() error) { done <- s() }(sd)
		select {
		case err := <-done:
			if err != nil {
				t.Errorf("shutdown error: %v", err)
			}
		case <-time.After(10 * time.Second):
			t.Error("shutdown timed out")
		}
	}
}

// ── сценарий 6: невалидный конфиг при старте ─────────────────────────────────

// TestE2E_InvalidConfigFailsFast проверяет что невалидный конфиг не даёт стартовать,
// и платформа возвращает явную ошибку, а не падает с паникой.
func TestE2E_InvalidConfigFailsFast(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	writeCfg(t, cfgPath, []byte(`
app:
  env: local
db:
  host: localhost
  port: 5432
  name: tenex
redis:
  addr: "localhost:6379"
scheduler:
  max_retries: 3
  reminder_retry_interval: 30s
`))

	_, cfgShutdown, err := config.Bootstrap(context.Background(), cfgPath)
	if err == nil {
		cfgShutdown()
		t.Fatal("expected Bootstrap to fail with invalid config, got nil")
	}

	if cfgShutdown != nil {
		cfgShutdown()
	}
	t.Logf("correctly rejected invalid config: %v", err)
}
