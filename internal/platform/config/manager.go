package config

import (
	"fmt"
	"os"
	"sync"
	"sync/atomic"
)

type ChangeListener func(old, new *AppConfig)

type listenerEntry struct {
	id uint64
	fn ChangeListener
}

type Manager struct {
	mu        sync.RWMutex
	cfg       atomic.Pointer[AppConfig]
	loader    *Loader
	listeners []listenerEntry
	nextID    atomic.Uint64

	reloadCh  chan struct{}
	stopCh    chan struct{}
	closeOnce sync.Once
	wg        sync.WaitGroup
}

func NewManager(loader *Loader) *Manager {
	m := &Manager{
		loader:   loader,
		reloadCh: make(chan struct{}, 1),
		stopCh:   make(chan struct{}),
	}
	m.wg.Add(1)
	go m.notifyWorker()
	return m
}

func (m *Manager) Close() {
	m.closeOnce.Do(func() { close(m.stopCh) })
	m.wg.Wait()
}

func (m *Manager) AddListener(fn ChangeListener) (unsubscribe func()) {
	id := m.nextID.Add(1)

	m.mu.Lock()
	m.listeners = append(m.listeners, listenerEntry{id: id, fn: fn})
	m.mu.Unlock()

	return func() {
		m.mu.Lock()
		defer m.mu.Unlock()
		for i, e := range m.listeners {
			if e.id == id {
				last := len(m.listeners) - 1
				m.listeners[i] = m.listeners[last]
				m.listeners[last] = listenerEntry{}
				m.listeners = m.listeners[:last]
				return
			}
		}
	}
}

func (m *Manager) Load() error {
	cfg, err := m.loader.Load()
	if err != nil {
		return err
	}
	m.cfg.Store(cfg)

	select {
	case m.reloadCh <- struct{}{}:
	default:
	}
	return nil
}

func (m *Manager) Get() *AppConfig {
	return m.cfg.Load()
}

func (m *Manager) notifyWorker() {
	defer m.wg.Done()
	var prev *AppConfig

	deliver := func() {
		current := m.cfg.Load()

		m.mu.RLock()
		listeners := make([]listenerEntry, len(m.listeners))
		copy(listeners, m.listeners)
		m.mu.RUnlock()

		for _, e := range listeners {
			fn := e.fn
			func() {
				defer func() {
					if r := recover(); r != nil {
						fmt.Fprintf(os.Stderr, "[config] listener panic: %v\n", r)
					}
				}()
				fn(prev, current)
			}()
		}
		prev = current
	}

	for {
		select {
		case <-m.stopCh:
			select {
			case <-m.reloadCh:
				deliver()
			default:
			}
			return
		case <-m.reloadCh:
			deliver()
		}
	}
}
