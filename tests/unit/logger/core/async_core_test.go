package core_unit

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"go.uber.org/zap/zapcore"

	"github.com/jsol/tenex-platform/internal/platform/logger/core"
)

type recordingCore struct {
	mu      sync.Mutex
	entries []recordedEntry
}

type recordedEntry struct {
	entry  zapcore.Entry
	fields []zapcore.Field
}

func (r *recordingCore) Enabled(zapcore.Level) bool { return true }
func (r *recordingCore) With(f []zapcore.Field) zapcore.Core {
	return &withRecordingCore{parent: r, fields: append([]zapcore.Field{}, f...)}
}
func (r *recordingCore) Check(e zapcore.Entry, ce *zapcore.CheckedEntry) *zapcore.CheckedEntry {
	return ce.AddCore(e, r)
}
func (r *recordingCore) Write(e zapcore.Entry, fields []zapcore.Field) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	cp := make([]zapcore.Field, len(fields))
	copy(cp, fields)
	r.entries = append(r.entries, recordedEntry{e, cp})
	return nil
}
func (r *recordingCore) Sync() error { return nil }
func (r *recordingCore) len() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.entries)
}
func (r *recordingCore) get(i int) recordedEntry {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.entries[i]
}

type withRecordingCore struct {
	parent *recordingCore
	fields []zapcore.Field
}

func (w *withRecordingCore) Enabled(zapcore.Level) bool { return true }
func (w *withRecordingCore) With(f []zapcore.Field) zapcore.Core {
	merged := append(append([]zapcore.Field{}, w.fields...), f...)
	return &withRecordingCore{parent: w.parent, fields: merged}
}
func (w *withRecordingCore) Check(e zapcore.Entry, ce *zapcore.CheckedEntry) *zapcore.CheckedEntry {
	return ce.AddCore(e, w)
}
func (w *withRecordingCore) Write(e zapcore.Entry, f []zapcore.Field) error {
	merged := append(append([]zapcore.Field{}, w.fields...), f...)
	return w.parent.Write(e, merged)
}
func (w *withRecordingCore) Sync() error { return nil }

func field(key, val string) zapcore.Field {
	return zapcore.Field{Key: key, String: val, Type: zapcore.StringType}
}
func entry(msg string) zapcore.Entry {
	return zapcore.Entry{Level: zapcore.InfoLevel, Message: msg}
}

// TestAsyncCore_WriteAndDrain — записи доходят до inner core после Close().
func TestAsyncCore_WriteAndDrain(t *testing.T) {
	rec := &recordingCore{}
	ac := core.NewAsyncCore(rec, 64)

	const n = 20
	for i := 0; i < n; i++ {
		if err := ac.Write(entry(fmt.Sprintf("msg-%d", i)), nil); err != nil {
			t.Fatalf("Write[%d] failed: %v", i, err)
		}
	}
	if err := ac.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}
	if got := rec.len(); got != n {
		t.Errorf("expected %d entries, got %d", n, got)
	}
}

func TestAsyncCore_CloseNoPanic(t *testing.T) {
	rec := &recordingCore{}
	ac := core.NewAsyncCore(rec, 64)

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_ = ac.Write(entry(fmt.Sprintf("concurrent-%d", i)), nil)
		}(i)
	}
	if err := ac.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}
	wg.Wait()
}

func TestAsyncCore_CloseIdempotent(t *testing.T) {
	rec := &recordingCore{}
	ac := core.NewAsyncCore(rec, 64)
	_ = ac.Write(entry("hello"), nil)

	if err := ac.Close(); err != nil {
		t.Fatalf("first Close failed: %v", err)
	}
	// bgDone внутри once.Do — вторая горутина не создаётся.
	if err := ac.Close(); err != nil {
		t.Fatalf("second Close failed: %v", err)
	}
}

// TestAsyncCore_WriteAfterCloseSilentlyDropped — запись после Close не паникует,
// возвращает nil, но запись НЕ доходит до inner core.
func TestAsyncCore_WriteAfterCloseSilentlyDropped(t *testing.T) {
	rec := &recordingCore{}
	ac := core.NewAsyncCore(rec, 64)
	_ = ac.Write(entry("before"), nil)
	_ = ac.Close()

	if err := ac.Write(entry("after-close"), nil); err != nil {
		t.Fatalf("Write after Close should return nil, got: %v", err)
	}
	if rec.len() != 1 {
		t.Errorf("only 1 entry expected (before-close), got %d", rec.len())
	}
}

func TestAsyncCore_WithFieldsPreserved(t *testing.T) {
	rec := &recordingCore{}
	ac := core.NewAsyncCore(rec, 64)

	child := ac.With([]zapcore.Field{field("user_id", "u123")})
	_ = child.Write(entry("event"), []zapcore.Field{field("trace_id", "t456")})
	_ = ac.Close()

	if rec.len() != 1 {
		t.Fatalf("expected 1 entry, got %d", rec.len())
	}
	got := rec.get(0)
	hasUser, hasTrace := false, false
	for _, f := range got.fields {
		if f.Key == "user_id" && f.String == "u123" {
			hasUser = true
		}
		if f.Key == "trace_id" && f.String == "t456" {
			hasTrace = true
		}
	}
	if !hasUser {
		t.Error("user_id from With() is missing — H-7 regression")
	}
	if !hasTrace {
		t.Error("trace_id call-site field is missing")
	}
}

// TestAsyncCore_NestedWithChain — поля накапливаются по всей цепочке With().
func TestAsyncCore_NestedWithChain(t *testing.T) {
	rec := &recordingCore{}
	ac := core.NewAsyncCore(rec, 64)

	a := ac.With([]zapcore.Field{field("a", "1")})
	b := a.With([]zapcore.Field{field("b", "2")})
	c := b.With([]zapcore.Field{field("c", "3")})
	_ = c.Write(entry("nested"), nil)
	_ = ac.Close()

	if rec.len() != 1 {
		t.Fatalf("expected 1 entry, got %d", rec.len())
	}
	keys := map[string]bool{}
	for _, f := range rec.get(0).fields {
		keys[f.Key] = true
	}
	for _, k := range []string{"a", "b", "c"} {
		if !keys[k] {
			t.Errorf("field %q missing from nested With() chain", k)
		}
	}
}

// TestAsyncCore_QueueFullDrops — при заполненной очереди записи дропаются без блокировки.
func TestAsyncCore_QueueFullDrops(t *testing.T) {
	slow := &slowCore{delay: 5 * time.Millisecond}
	ac := core.NewAsyncCore(slow, 2)

	for i := 0; i < 500; i++ {
		if err := ac.Write(entry("drop-test"), nil); err != nil {
			t.Fatalf("Write returned error: %v", err)
		}
	}
	_ = ac.Close()
}

// TestAsyncCore_ConcurrentWriteClose — race detector: 100 горутин пишут,
// Close вызывается конкурентно. Запускать с -race.
func TestAsyncCore_ConcurrentWriteClose(t *testing.T) {
	rec := &recordingCore{}
	ac := core.NewAsyncCore(rec, 1024)

	var wg sync.WaitGroup
	stop := make(chan struct{})

	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				select {
				case <-stop:
					return
				default:
					_ = ac.Write(entry(fmt.Sprintf("%d-%d", i, j)), nil)
				}
			}
		}(i)
	}

	time.AfterFunc(5*time.Millisecond, func() {
		close(stop)
		_ = ac.Close()
	})
	wg.Wait()
}

type slowCore struct{ delay time.Duration }

func (s *slowCore) Enabled(zapcore.Level) bool { return true }
func (s *slowCore) With(f []zapcore.Field) zapcore.Core {
	return &withRecordingCore{parent: &recordingCore{}, fields: f}
}
func (s *slowCore) Check(e zapcore.Entry, ce *zapcore.CheckedEntry) *zapcore.CheckedEntry {
	return ce.AddCore(e, s)
}
func (s *slowCore) Write(zapcore.Entry, []zapcore.Field) error {
	time.Sleep(s.delay)
	return nil
}
func (s *slowCore) Sync() error { return nil }
