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
	mu        sync.Mutex
	cfg       atomic.Pointer[AppConfig]
	prev      *AppConfig
	loader    *Loader
	listeners []listenerEntry
	nextID    atomic.Uint64
	//notifying atomic.Int32
}

func NewManager(loader *Loader) *Manager {
	return &Manager{loader: loader}
}

func (m *Manager) Close() {}

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

	m.mu.Lock()
	prev := m.prev
	m.prev = cfg
	snapshot := make([]listenerEntry, len(m.listeners))
	copy(snapshot, m.listeners)
	m.mu.Unlock()

	m.cfg.Store(cfg)

	//if m.notifying.CompareAndSwap(0, 1) {
	//	go func() {
	//		defer m.notifying.Store(0)
	//		for _, e := range snapshot {
	//			fn := e.fn
	//			func() {
	//				defer func() {
	//					if r := recover(); r != nil {
	//						fmt.Fprintf(os.Stderr, "[config] listener panic: %v\n", r)
	//					}
	//				}()
	//				fn(prev, cfg)
	//			}()
	//		}
	//	}()
	if len(snapshot) > 0 {
		go notifyListeners(snapshot, prev, cfg)
	}

	return nil
}

func notifyListeners(snapshot []listenerEntry, prev, cfg *AppConfig) {
	for _, e := range snapshot {
		fn := e.fn
		func() {
			defer func() {
				if r := recover(); r != nil {
					fmt.Fprintf(os.Stderr, "[config] listener panic: %v\n", r)
				}
			}()
			fn(prev, cfg)
		}()
	}
}

func (m *Manager) Get() *AppConfig {
	return m.cfg.Load()
}
