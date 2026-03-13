package logger_integration

import (
	"sync"
	"testing"
	"time"

	"go.uber.org/zap/zapcore"

	"github.com/jsolteam/tenex-platform/internal/platform/config"
	"github.com/jsolteam/tenex-platform/internal/platform/logger"
	logcore "github.com/jsolteam/tenex-platform/internal/platform/logger/core"
)

type memProvider struct {
	mu   sync.Mutex
	data map[string]interface{}
}

func (p *memProvider) Name() string     { return "mem" }
func (p *memProvider) IsRequired() bool { return true }
func (p *memProvider) Load() (map[string]interface{}, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	cp := make(map[string]interface{}, len(p.data))
	for k, v := range p.data {
		cp[k] = v
	}
	return cp, nil
}
func (p *memProvider) Set(k string, v interface{}) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.data[k] = v
}

func baseSettings() map[string]interface{} {
	return map[string]interface{}{
		"app.env":                           "local",
		"app.name":                          "test-app",
		"app.log_level":                     "info",
		"db.host":                           "localhost",
		"db.port":                           "5432",
		"db.user":                           "tenex",
		"db.name":                           "tenex",
		"redis.addr":                        "localhost:6379",
		"scheduler.max_retries":             "3",
		"scheduler.reminder_retry_interval": "30s",
	}
}

func newTestStack(t *testing.T) (*config.Manager, *memProvider, func() error) {
	t.Helper()
	p := &memProvider{data: baseSettings()}
	loader := config.NewLoader(p)
	mgr := config.NewManager(loader)
	if err := mgr.Load(); err != nil {
		t.Fatalf("mgr.Load: %v", err)
	}
	shutdown, err := logger.Bootstrap(mgr)
	if err != nil {
		mgr.Close()
		t.Fatalf("logger.Bootstrap: %v", err)
	}
	t.Cleanup(func() {
		_ = shutdown()
		mgr.Close()
	})
	return mgr, p, shutdown
}

// TestBootstrap_NoError — Bootstrap завершается без ошибки, shutdown возвращает nil.
func TestBootstrap_NoError(t *testing.T) {
	_, _, shutdown := newTestStack(t)

	done := make(chan error, 1)
	go func() { done <- shutdown() }()

	select {
	case err := <-done:
		if err != nil {
			t.Errorf("shutdown returned: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("shutdown timed out — AsyncCore.Close() blocked")
	}
}

func TestBootstrap_ShutdownUnsubscribes(t *testing.T) {
	mgr, _, shutdown := newTestStack(t)

	if err := shutdown(); err != nil {
		t.Fatalf("shutdown: %v", err)
	}

	// Trigger config reload ПОСЛЕ закрытия logger'а — listener должен быть отписан.
	for i := 0; i < 10; i++ {
		_ = mgr.Load()
		time.Sleep(10 * time.Millisecond)
	}
	time.Sleep(100 * time.Millisecond)
	// Нет паники — тест прошёл.
}

// TestBootstrap_LevelHotReload — смена log_level через config не вызывает ошибок.
func TestBootstrap_LevelHotReload(t *testing.T) {
	mgr, p, _ := newTestStack(t)

	p.Set("app.log_level", "debug")
	if err := mgr.Load(); err != nil {
		t.Fatalf("reload: %v", err)
	}
	time.Sleep(150 * time.Millisecond)
	// Нет ошибок, нет паники — тест прошёл.
}

// TestBootstrap_ConcurrentShutdown — несколько конкурентных shutdown безопасны.
func TestBootstrap_ConcurrentShutdown(t *testing.T) {
	_, _, shutdown := newTestStack(t)

	var wg sync.WaitGroup
	errs := make(chan error, 10)
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			errs <- shutdown()
		}()
	}

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
		close(errs)
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("concurrent shutdown timed out")
	}
	for err := range errs {
		if err != nil {
			t.Errorf("shutdown error: %v", err)
		}
	}
}

// TestParseLevel_AllVariants — ParseLevel корректно разбирает все уровни.
func TestParseLevel_AllVariants(t *testing.T) {
	tests := []struct {
		input string
		want  zapcore.Level
	}{
		{"debug", zapcore.DebugLevel},
		{"info", zapcore.InfoLevel},
		{"warn", zapcore.WarnLevel},
		{"error", zapcore.ErrorLevel},
		{"", zapcore.InfoLevel},        // дефолт
		{"INVALID", zapcore.InfoLevel}, // дефолт при неверном значении
		{"INFO", zapcore.InfoLevel},    // регистронезависимый
		{"DEBUG", zapcore.DebugLevel},
	}
	for _, tt := range tests {
		got := logcore.ParseLevel(tt.input)
		if got != tt.want {
			t.Errorf("ParseLevel(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}
