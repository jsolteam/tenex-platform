package config

import (
	"context"
	"fmt"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/fsnotify/fsnotify"
)

const (
	rewatchMaxAttempts = 10
	rewatchBaseDelay   = 100 * time.Millisecond
	rewatchMaxDelay    = 2 * time.Second
)

type Watcher struct {
	path     string
	manager  *Manager
	debounce time.Duration
}

func NewWatcher(path string, m *Manager) *Watcher {
	return &Watcher{
		path:     path,
		manager:  m,
		debounce: 300 * time.Millisecond,
	}
}

func (w *Watcher) Run(ctx context.Context) error {
	fw, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("fsnotify: %w", err)
	}

	var goroutineWg sync.WaitGroup
	defer func() {
		goroutineWg.Wait()
		fw.Close()
	}()

	if err := fw.Add(w.path); err != nil {
		return fmt.Errorf("watch %s: %w", w.path, err)
	}

	var (
		debounceTimer *time.Timer
		timerC        <-chan time.Time
		reloadMu      sync.Mutex
	)

	stopTimer := func() {
		if debounceTimer == nil {
			return
		}
		if !debounceTimer.Stop() {
			select {
			case <-debounceTimer.C:
			default:
			}
		}
		debounceTimer = nil
		timerC = nil
	}

	scheduleReload := func() {
		stopTimer()
		debounceTimer = time.NewTimer(w.debounce)
		timerC = debounceTimer.C
	}

	doReload := func() {
		if !reloadMu.TryLock() {
			return
		}
		defer reloadMu.Unlock()
		if err := w.manager.Load(); err != nil {
			fmt.Fprintf(os.Stderr, "[config] reload failed: %v\n", err)
		} else {
			fmt.Fprintf(os.Stderr, "[config] reloaded from %s\n", w.path)
		}
	}
	
	var pendingRewatches atomic.Int64
	rewatchDone := make(chan struct{}, 1)

	goroutineWg.Add(1)
	go func() {
		defer goroutineWg.Done()
		for {
			select {
			case <-ctx.Done():
				return
			default:
			}

			if pendingRewatches.Load() > 0 {
				w.rewatchWithBackoff(ctx, fw, rewatchDone)
				pendingRewatches.Store(0)
				continue
			}

			select {
			case <-ctx.Done():
				return
			case <-time.After(10 * time.Millisecond):
			}
		}
	}()

	for {
		select {
		case <-ctx.Done():
			stopTimer()
			return ctx.Err()

		case <-timerC:
			timerC = nil
			debounceTimer = nil
			doReload()

		case <-rewatchDone:
			scheduleReload()

		case event, ok := <-fw.Events:
			if !ok {
				return nil
			}
			switch {
			case event.Has(fsnotify.Write) || event.Has(fsnotify.Create):
				scheduleReload()
			case event.Has(fsnotify.Remove) || event.Has(fsnotify.Rename):
				stopTimer()
				pendingRewatches.Add(1)
			}

		case err, ok := <-fw.Errors:
			if !ok {
				return nil
			}
			fmt.Fprintf(os.Stderr, "[config] watcher error: %v\n", err)
		}
	}
}

func (w *Watcher) rewatchWithBackoff(
	ctx context.Context,
	fw *fsnotify.Watcher,
	rewatchDone chan<- struct{},
) {
	delay := rewatchBaseDelay
	for attempt := 0; attempt < rewatchMaxAttempts; attempt++ {
		select {
		case <-ctx.Done():
			return
		case <-time.After(delay):
		}

		if err := fw.Add(w.path); err == nil {
			select {
			case rewatchDone <- struct{}{}:
			default:
			}
			return
		}

		delay *= 2
		if delay > rewatchMaxDelay {
			delay = rewatchMaxDelay
		}
	}
	fmt.Fprintf(os.Stderr,
		"[config] failed to re-watch %s after %d attempts; hot-reload disabled until restart\n",
		w.path, rewatchMaxAttempts,
	)
}
