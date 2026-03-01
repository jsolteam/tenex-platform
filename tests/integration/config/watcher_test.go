package config_integration

import (
	"context"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jsol/tenex-platform/internal/platform/config"
)

// validYAML возвращает минимально корректный YAML-конфиг.
func validYAML() []byte {
	return []byte(`
app:
  env: test
  log_level: info
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

func writeCfg(t *testing.T, path string) {
	t.Helper()
	if err := os.WriteFile(path, validYAML(), 0o644); err != nil {
		t.Fatalf("writeCfg: %v", err)
	}
}

// TestWatcher_DetectsWrite — watcher вызывает reload при записи файла.
func TestWatcher_DetectsWrite(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	writeCfg(t, cfgPath)

	mgr, shutdown, err := config.Bootstrap(context.Background(), cfgPath)
	if err != nil {
		t.Fatalf("Bootstrap: %v", err)
	}
	t.Cleanup(shutdown)

	var reloads atomic.Int64
	mgr.AddListener(func(_, _ *config.AppConfig) { reloads.Add(1) })

	time.Sleep(50 * time.Millisecond)
	writeCfg(t, cfgPath)

	waitFor(t, 2*time.Second, func() bool { return reloads.Load() >= 1 })
	if reloads.Load() == 0 {
		t.Error("watcher did not trigger reload after file write")
	}
}

// TestWatcher_DebounceCoalesces — быстрые изменения сворачиваются в немного reload'ов.
func TestWatcher_DebounceCoalesces(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	writeCfg(t, cfgPath)

	mgr, shutdown, err := config.Bootstrap(context.Background(), cfgPath)
	if err != nil {
		t.Fatalf("Bootstrap: %v", err)
	}
	t.Cleanup(shutdown)

	var reloads atomic.Int64
	mgr.AddListener(func(_, _ *config.AppConfig) { reloads.Add(1) })

	for i := 0; i < 10; i++ {
		writeCfg(t, cfgPath)
		time.Sleep(3 * time.Millisecond)
	}
	time.Sleep(800 * time.Millisecond)

	n := reloads.Load()
	if n == 0 {
		t.Error("no reloads triggered — debounce blocked everything")
	}
	if n > 5 {
		t.Errorf("too many reloads (%d) — debounce not working", n)
	}
}

func TestWatcher_RenameRecovery(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	writeCfg(t, cfgPath)

	mgr, shutdown, err := config.Bootstrap(context.Background(), cfgPath)
	if err != nil {
		t.Fatalf("Bootstrap: %v", err)
	}
	t.Cleanup(shutdown)

	var reloads atomic.Int64
	mgr.AddListener(func(_, _ *config.AppConfig) { reloads.Add(1) })

	tmp := cfgPath + ".tmp"
	writeCfg(t, tmp)
	if err := os.Rename(tmp, cfgPath); err != nil {
		t.Fatalf("rename: %v", err)
	}

	waitFor(t, 3*time.Second, func() bool { return reloads.Load() >= 1 })
	if reloads.Load() == 0 {
		t.Error("watcher did not reload after atomic rename — H-4 regression")
	}

	baseCount := reloads.Load()
	writeCfg(t, cfgPath)
	time.Sleep(700 * time.Millisecond)
	if reloads.Load() <= baseCount {
		t.Error("watcher stopped working after rename recovery")
	}
}

// TestWatcher_ContextCancel — отмена контекста завершает watcher без утечки горутин.
func TestWatcher_ContextCancel(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	writeCfg(t, cfgPath)

	ctx, cancel := context.WithCancel(context.Background())
	_, shutdown, err := config.Bootstrap(ctx, cfgPath)
	if err != nil {
		t.Fatalf("Bootstrap: %v", err)
	}

	cancel()

	done := make(chan struct{})
	go func() {
		shutdown()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("watcher did not stop after context cancel — goroutine leak")
	}
}

// TestWatcher_NoFile — без filePath watcher не создаётся, shutdown быстрый.
func TestWatcher_NoFile(t *testing.T) {
	_, shutdown, err := config.Bootstrap(context.Background(), "")
	if err != nil {
		t.Skipf("Bootstrap without file failed (expected in test env): %v", err)
	}
	done := make(chan struct{})
	go func() {
		shutdown()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("shutdown without watcher timed out")
	}
}
