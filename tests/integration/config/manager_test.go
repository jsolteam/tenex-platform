package config_integration

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jsolteam/tenex-platform/internal/platform/config"
)

type memProvider struct {
	mu   sync.Mutex
	data map[string]interface{}
}

func newMemProvider() *memProvider {
	return &memProvider{data: minSettings()}
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

func minSettings() map[string]interface{} {
	return map[string]interface{}{
		"db.host":                           "localhost",
		"db.port":                           "5432",
		"db.user":                           "tenex",
		"db.name":                           "tenex",
		"redis.addr":                        "localhost:6379",
		"scheduler.max_retries":             "3",
		"scheduler.reminder_retry_interval": "30s",
	}
}

func newTestManager(t *testing.T) (*config.Manager, *memProvider) {
	t.Helper()
	p := newMemProvider()
	loader := config.NewLoader(p)
	mgr := config.NewManager(loader)
	if err := mgr.Load(); err != nil {
		t.Fatalf("initial Load failed: %v", err)
	}
	t.Cleanup(mgr.Close)
	return mgr, p
}

// TestManager_GetAfterLoad — Get() возвращает корректный конфиг после Load().
func TestManager_GetAfterLoad(t *testing.T) {
	mgr, _ := newTestManager(t)

	cfg := mgr.Get()
	if cfg == nil {
		t.Fatal("Get() returned nil")
	}
	if cfg.DB.Host != "localhost" {
		t.Errorf("DB.Host = %q, want %q", cfg.DB.Host, "localhost")
	}
}

// TestManager_ListenerCalledOnReload — listener вызывается после Load().
func TestManager_ListenerCalledOnReload(t *testing.T) {
	mgr, _ := newTestManager(t)

	var called atomic.Int64
	mgr.AddListener(func(_, _ *config.AppConfig) {
		called.Add(1)
	})

	_ = mgr.Load()

	waitFor(t, 500*time.Millisecond, func() bool { return called.Load() >= 1 })
	if called.Load() == 0 {
		t.Error("listener was never called after Load()")
	}
}

func TestManager_Unsubscribe(t *testing.T) {
	mgr, _ := newTestManager(t)

	var countBefore, countAfter atomic.Int64
	unsub := mgr.AddListener(func(_, _ *config.AppConfig) { countBefore.Add(1) })

	_ = mgr.Load()
	waitFor(t, 300*time.Millisecond, func() bool { return countBefore.Load() >= 1 })
	beforeCount := countBefore.Load()

	// Отписываемся.
	unsub()

	// Новый listener — должен срабатывать.
	mgr.AddListener(func(_, _ *config.AppConfig) { countAfter.Add(1) })
	_ = mgr.Load()
	waitFor(t, 300*time.Millisecond, func() bool { return countAfter.Load() >= 1 })

	// Отписанный listener не должен был сработать повторно.
	if countBefore.Load() != beforeCount {
		t.Errorf("unsubscribed listener fired after unsubscribe: %d → %d",
			beforeCount, countBefore.Load())
	}
	if countAfter.Load() == 0 {
		t.Error("new listener was never called")
	}
}

// TestManager_MultipleListeners — все зарегистрированные listener'ы вызываются.
func TestManager_MultipleListeners(t *testing.T) {
	mgr, _ := newTestManager(t)

	const n = 5

	done := make([]chan struct{}, n)
	for i := range done {
		ch := make(chan struct{}, 1)
		done[i] = ch
		mgr.AddListener(func(_, _ *config.AppConfig) {
			select {
			case ch <- struct{}{}:
			default:
			}
		})
	}

	_ = mgr.Load()
	time.Sleep(200 * time.Millisecond)

	for i, ch := range done {
		select {
		case <-ch:
			// listener[i] сработал
		default:
			t.Errorf("listener[%d] was never called", i)
		}
	}
}

// TestManager_ListenerPanicIsolated — паника в одном listener не роняет worker.
func TestManager_ListenerPanicIsolated(t *testing.T) {
	mgr, _ := newTestManager(t)

	var safeCount atomic.Int64
	mgr.AddListener(func(_, _ *config.AppConfig) { panic("intentional") })
	mgr.AddListener(func(_, _ *config.AppConfig) { safeCount.Add(1) })

	_ = mgr.Load()
	waitFor(t, 500*time.Millisecond, func() bool { return safeCount.Load() >= 1 })

	if safeCount.Load() == 0 {
		t.Error("safe listener not called after panicking listener — worker killed")
	}
}

// TestManager_RapidReloads — H-3: множество быстрых reload не теряют сигнал.
func TestManager_RapidReloads(t *testing.T) {
	mgr, _ := newTestManager(t)

	var count atomic.Int64
	mgr.AddListener(func(_, _ *config.AppConfig) { count.Add(1) })

	for i := 0; i < 30; i++ {
		_ = mgr.Load()
	}
	time.Sleep(300 * time.Millisecond)

	if count.Load() == 0 {
		t.Error("listener never called during rapid reloads — H-3 signal lost")
	}
}

// TestManager_GetReturnsLatest — после изменения данных Get() отдаёт новое.
func TestManager_GetReturnsLatest(t *testing.T) {
	mgr, p := newTestManager(t)
	first := mgr.Get()

	p.Set("app.env", "production")
	_ = mgr.Load()
	second := mgr.Get()

	if first == second {
		t.Error("Get() returned same pointer after data change")
	}
	if second.App.Env != "production" {
		t.Errorf("App.Env = %q, want %q", second.App.Env, "production")
	}
}

// TestManager_ConcurrentLoadGet — race detector: конкурентные Load и Get безопасны.
func TestManager_ConcurrentLoadGet(t *testing.T) {
	mgr, _ := newTestManager(t)

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 30; j++ {
				_ = mgr.Load()
			}
		}()
	}
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 30; j++ {
				_ = mgr.Get()
				time.Sleep(time.Millisecond)
			}
		}()
	}
	wg.Wait()
}

// TestManager_CloseTerminatesWorker — Close() завершает notifyWorker без утечки.
func TestManager_CloseTerminatesWorker(t *testing.T) {
	p := newMemProvider()
	mgr := config.NewManager(config.NewLoader(p))
	_ = mgr.Load()

	done := make(chan struct{})
	go func() {
		mgr.Close()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("Close() timed out — notifyWorker goroutine leak")
	}
}
