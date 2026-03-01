package core

import (
	"fmt"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"go.uber.org/zap/zapcore"
)

const (
	DefaultAsyncQueueSize = 8192
	asyncCloseTimeout     = 5 * time.Second
)

type asyncEntry struct {
	entry  zapcore.Entry
	fields []zapcore.Field
}

type AsyncCore struct {
	inner zapcore.Core
	ch    chan asyncEntry
	done  chan struct{}

	once   sync.Once
	bgDone chan struct{}

	closeMu  sync.RWMutex
	closed   bool
	writerWg sync.WaitGroup

	bgWg    sync.WaitGroup
	dropped atomic.Int64
}

var _ zapcore.Core = (*AsyncCore)(nil)

func NewAsyncCore(inner zapcore.Core, queueSize int) *AsyncCore {
	if queueSize <= 0 {
		queueSize = DefaultAsyncQueueSize
	}
	a := &AsyncCore{
		inner:  inner,
		ch:     make(chan asyncEntry, queueSize),
		done:   make(chan struct{}),
		bgDone: make(chan struct{}),
	}
	a.bgWg.Add(2)
	go a.drain()
	go a.reportDropped()
	return a
}

func (a *AsyncCore) drain() {
	defer a.bgWg.Done()
	for e := range a.ch {
		func() {
			defer func() {
				if r := recover(); r != nil {
					fmt.Fprintf(os.Stderr, "[logger] async drain panic: %v\n", r)
				}
			}()
			_ = a.inner.Write(e.entry, e.fields)
		}()
	}
}

func (a *AsyncCore) reportDropped() {
	defer a.bgWg.Done()
	t := time.NewTicker(10 * time.Second)
	defer t.Stop()
	for {
		select {
		case <-t.C:
			if n := a.dropped.Swap(0); n > 0 {
				fmt.Fprintf(os.Stderr, "[logger] dropped %d log entries (queue full)\n", n)
			}
		case <-a.done:
			if n := a.dropped.Swap(0); n > 0 {
				fmt.Fprintf(os.Stderr, "[logger] dropped %d log entries (queue full)\n", n)
			}
			return
		}
	}
}

func (a *AsyncCore) Close() error {
	a.once.Do(func() {
		a.closeMu.Lock()
		a.closed = true
		a.closeMu.Unlock()

		a.writerWg.Wait()

		close(a.ch)
		close(a.done)

		go func() {
			a.bgWg.Wait()
			close(a.bgDone)
		}()
	})

	select {
	case <-a.bgDone:
	case <-time.After(asyncCloseTimeout):
		fmt.Fprintf(os.Stderr,
			"[logger] async close timed out after %s; some log entries may be lost\n",
			asyncCloseTimeout,
		)
	}

	return a.inner.Sync()
}

func (a *AsyncCore) Enabled(l zapcore.Level) bool { return a.inner.Enabled(l) }

func (a *AsyncCore) With(fields []zapcore.Field) zapcore.Core {
	cp := make([]zapcore.Field, len(fields))
	copy(cp, fields)
	return &asyncChildCore{
		parent:      a,
		inner:       a.inner.With(fields),
		extraFields: cp,
	}
}

func (a *AsyncCore) Check(e zapcore.Entry, ce *zapcore.CheckedEntry) *zapcore.CheckedEntry {
	if a.Enabled(e.Level) {
		return ce.AddCore(e, a)
	}
	return ce
}

func (a *AsyncCore) Write(e zapcore.Entry, fields []zapcore.Field) error {
	a.closeMu.RLock()
	if a.closed {
		a.closeMu.RUnlock()
		return nil
	}
	a.writerWg.Add(1)
	a.closeMu.RUnlock()
	defer a.writerWg.Done()

	cp := make([]zapcore.Field, len(fields))
	copy(cp, fields)

	select {
	case a.ch <- asyncEntry{entry: e, fields: cp}:
	default:
		a.dropped.Add(1)
	}
	return nil
}

func (a *AsyncCore) Sync() error { return nil }

type asyncChildCore struct {
	parent      *AsyncCore
	inner       zapcore.Core
	extraFields []zapcore.Field
}

var _ zapcore.Core = (*asyncChildCore)(nil)

func (c *asyncChildCore) Enabled(l zapcore.Level) bool { return c.inner.Enabled(l) }

func (c *asyncChildCore) With(fields []zapcore.Field) zapcore.Core {
	merged := make([]zapcore.Field, len(c.extraFields)+len(fields))
	copy(merged, c.extraFields)
	copy(merged[len(c.extraFields):], fields)
	return &asyncChildCore{
		parent:      c.parent,
		inner:       c.inner.With(fields),
		extraFields: merged,
	}
}

func (c *asyncChildCore) Check(e zapcore.Entry, ce *zapcore.CheckedEntry) *zapcore.CheckedEntry {
	if c.Enabled(e.Level) {
		return ce.AddCore(e, c)
	}
	return ce
}

func (c *asyncChildCore) Write(e zapcore.Entry, fields []zapcore.Field) error {
	merged := make([]zapcore.Field, 0, len(c.extraFields)+len(fields))
	merged = append(merged, c.extraFields...)
	merged = append(merged, fields...)
	return c.parent.Write(e, merged)
}

func (c *asyncChildCore) Sync() error { return nil }
