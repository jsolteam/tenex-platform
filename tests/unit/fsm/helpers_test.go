package fsm_unit

import (
	"context"
	"sync"
	"time"

	"github.com/jsolteam/tenex-platform/internal/fsm"
)

// inMemoryStore — in-memory реализация fsm.Store для unit-тестов.
// Поддерживает TTL через time.AfterFunc.
type inMemoryStore struct {
	mu       sync.RWMutex
	data     map[string]*fsm.RawFSMState
	timers   map[string]*time.Timer
	fixedTTL time.Duration // 0 = использовать переданный TTL
}

func newInMemoryStore() *inMemoryStore {
	return &inMemoryStore{
		data:   make(map[string]*fsm.RawFSMState),
		timers: make(map[string]*time.Timer),
	}
}

// newInMemoryStoreWithTTL создаёт store с фиксированным TTL — удобно для тестов истечения.
func newInMemoryStoreWithTTL(ttl time.Duration) *inMemoryStore {
	return &inMemoryStore{
		data:     make(map[string]*fsm.RawFSMState),
		timers:   make(map[string]*time.Timer),
		fixedTTL: ttl,
	}
}

func (s *inMemoryStore) key(messenger, userID string) string {
	return messenger + ":" + userID
}

func (s *inMemoryStore) Get(_ context.Context, messenger, userID string) (*fsm.RawFSMState, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	v, ok := s.data[s.key(messenger, userID)]
	if !ok {
		return nil, nil
	}
	return v, nil
}

func (s *inMemoryStore) Set(_ context.Context, messenger, userID string, state *fsm.RawFSMState, ttl time.Duration) error {
	k := s.key(messenger, userID)
	s.mu.Lock()
	defer s.mu.Unlock()

	s.data[k] = state

	// Останавливаем старый таймер
	if t, ok := s.timers[k]; ok {
		t.Stop()
	}

	effectiveTTL := ttl
	if s.fixedTTL > 0 {
		effectiveTTL = s.fixedTTL
	}

	if effectiveTTL > 0 {
		s.timers[k] = time.AfterFunc(effectiveTTL, func() {
			s.mu.Lock()
			delete(s.data, k)
			delete(s.timers, k)
			s.mu.Unlock()
		})
	}
	return nil
}

func (s *inMemoryStore) Delete(_ context.Context, messenger, userID string) error {
	k := s.key(messenger, userID)
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.data, k)
	if t, ok := s.timers[k]; ok {
		t.Stop()
		delete(s.timers, k)
	}
	return nil
}
