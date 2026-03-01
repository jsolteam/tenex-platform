package config

import (
	"sync"
)

type Manager struct {
	mu     sync.RWMutex
	config *AppConfig
	loader *Loader
}

func NewManager(loader *Loader) *Manager {
	return &Manager{loader: loader}
}

func (m *Manager) Load() error {
	cfg, err := m.loader.Load()
	if err != nil {
		return err
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	m.config = cfg

	return nil
}

func (m *Manager) Get() *AppConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.config
}
