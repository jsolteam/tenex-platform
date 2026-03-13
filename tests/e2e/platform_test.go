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

	"github.com/jsolteam/tenex-platform/internal/platform/config"
	"github.com/jsolteam/tenex-platform/internal/platform/logger"
	"github.com/jsolteam/tenex-platform/internal/platform/logger/facade"
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

// TestE2E_PlatformStartsAndStops verifies:
//  1. config.Bootstrap reads a YAML file
//  2. logger.Bootstrap initialises the logger
//  3. facade.L() returns a working logger
//  4. Graceful shutdown completes without timeout
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

	l := facade.L()
	if l == nil {
		t.Fatal("facade.L() returned nil after bootstrap")
	}
	l.Info("e2e: platform started", zap.String("test", t.Name()))

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

// TestE2E_HotReloadConfigFile verifies:
//  1. Changing the YAML file triggers the watcher
//  2. Manager.Load() is called and listeners notified
//  3. Platform continues operating after reload
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

	time.Sleep(50 * time.Millisecond)
	writeCfg(t, cfgPath, configYAML("staging", "debug"))

	if !waitFor(t, 3*time.Second, func() bool { return reloads.Load() >= 1 }) {
		t.Fatal("config hot-reload did not trigger after file change")
	}

	cfg := mgr.Get()
	if cfg.App.Env != "staging" {
		t.Errorf("after reload App.Env = %q, want %q", cfg.App.Env, "staging")
	}

	facade.L().Info("e2e: after hot-reload", zap.String("env", cfg.App.Env))
}

// TestE2E_AtomicFileReplaceRecovers verifies watcher survives atomic rename.
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

	time.Sleep(50 * time.Millisecond)

	var reloads atomic.Int64
	mgr.AddListener(func(_, newCfg *config.AppConfig) {
		if newCfg != nil && newCfg.App.Env == "production" {
			reloads.Add(1)
		}
	})

	tmpPath := cfgPath + ".new"
	writeCfg(t, tmpPath, configYAML("production", "warn"))
	if err := os.Rename(tmpPath, cfgPath); err != nil {
		t.Fatalf("atomic rename: %v", err)
	}

	if !waitFor(t, 4*time.Second, func() bool { return reloads.Load() >= 1 }) {
		t.Fatal("watcher did not recover after atomic file replace")
	}

	cfg := mgr.Get()
	if cfg.App.Env != "production" {
		t.Errorf("after atomic replace App.Env = %q, want %q", cfg.App.Env, "production")
	}
}

// TestE2E_LoggerWithLokiEndpoint verifies Loki writer flushes on shutdown.
func TestE2E_LoggerWithLokiEndpoint(t *testing.T) {
	var lokiHits atomic.Int64
	lokiSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		lokiHits.Add(1)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer lokiSrv.Close()

	t.Setenv("LOKI_ENDPOINT", lokiSrv.URL)

	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	writeCfg(t, cfgPath, configYAML("local", "info"))

	mgr, cfgShutdown, err := config.Bootstrap(context.Background(), cfgPath)
	if err != nil {
		t.Fatalf("config.Bootstrap: %v", err)
	}
	defer cfgShutdown()

	if got := mgr.Get().Observability.LokiEndpoint; got != lokiSrv.URL {
		t.Fatalf("LokiEndpoint = %q, want %q", got, lokiSrv.URL)
	}

	logShutdown, err := logger.Bootstrap(mgr)
	if err != nil {
		t.Fatalf("logger.Bootstrap: %v", err)
	}

	l := facade.L()
	for i := 0; i < 5; i++ {
		l.Info("e2e-loki-test", zap.Int("i", i))
	}

	if err := logShutdown(); err != nil {
		t.Errorf("logShutdown: %v", err)
	}

	if lokiHits.Load() == 0 {
		t.Error("Loki received 0 requests after graceful shutdown — flush missing")
	}
}

// TestE2E_ConcurrentBootstraps verifies no race conditions across parallel bootstraps.
func TestE2E_ConcurrentBootstraps(t *testing.T) {
	const n = 3
	errs := make(chan error, n)
	shutdowns := make(chan func() error, n)

	for i := 0; i < n; i++ {
		go func() {
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
		}()
	}

	for i := 0; i < n; i++ {
		if err := <-errs; err != nil {
			t.Errorf("bootstrap failed: %v", err)
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

// TestE2E_InvalidConfigFailsFast verifies that an invalid config returns an error.
func TestE2E_InvalidConfigFailsFast(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	// db.user is missing — should fail validation.
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
	t.Logf("correctly rejected invalid config: %v", err)
}
