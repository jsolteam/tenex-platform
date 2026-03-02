package shutdown

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"
)

type Hook struct {
	Name string
	Fn   func(ctx context.Context) error
}

type Manager struct {
	timeout time.Duration
	hooks   []Hook

	triggerOnce sync.Once
	trigger     chan struct{}

	runOnce sync.Once
	runErr  error
}

func New(timeout time.Duration) *Manager {
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	return &Manager{
		timeout: timeout,
		trigger: make(chan struct{}),
	}
}

func (m *Manager) Register(name string, fn func(ctx context.Context) error) {
	m.hooks = append(m.hooks, Hook{Name: name, Fn: fn})
}

func (m *Manager) Wait(ctx context.Context) error {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)
	defer signal.Stop(sigCh)

	select {
	case sig := <-sigCh:
		fmt.Fprintf(os.Stderr, "[shutdown] signal: %s\n", sig)
	case <-ctx.Done():
		fmt.Fprintf(os.Stderr, "[shutdown] context done: %v\n", ctx.Err())
	case <-m.trigger:
	}

	return m.execute()
}

func (m *Manager) Stop() error {
	m.triggerOnce.Do(func() { close(m.trigger) })
	return m.execute()
}

func (m *Manager) execute() error {
	m.runOnce.Do(func() {
		m.runErr = m.runHooks()
	})
	return m.runErr
}

func (m *Manager) runHooks() error {
	ctx, cancel := context.WithTimeout(context.Background(), m.timeout)
	defer cancel()

	var errs []error

	for _, h := range m.hooks {
		if ctx.Err() != nil {
			errs = append(errs, fmt.Errorf("shutdown timeout: skipping hook %q", h.Name))
			break
		}

		if err := m.runOne(ctx, h); err != nil {
			errs = append(errs, err)
		}
	}

	return errors.Join(errs...)
}

func (m *Manager) runOne(ctx context.Context, h Hook) (retErr error) {
	defer func() {
		if r := recover(); r != nil {
			retErr = fmt.Errorf("hook %q panicked: %v", h.Name, r)
			fmt.Fprintf(os.Stderr, "[shutdown] %v\n", retErr)
		}
	}()

	fmt.Fprintf(os.Stderr, "[shutdown] stopping %q\n", h.Name)

	if err := h.Fn(ctx); err != nil {
		return fmt.Errorf("hook %q: %w", h.Name, err)
	}

	fmt.Fprintf(os.Stderr, "[shutdown] stopped  %q\n", h.Name)
	return nil
}
