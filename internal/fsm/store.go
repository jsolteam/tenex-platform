package fsm

import (
	"context"
	"encoding/json"
	"time"

	infraredis "github.com/jsolteam/tenex-platform/internal/infrastructure/redis"
	"github.com/jsolteam/tenex-platform/internal/platform/logger/core"
	"github.com/jsolteam/tenex-platform/internal/platform/observability/metrics"
	"github.com/jsolteam/tenex-platform/internal/platform/observability/tracing"
)

// defaultFSMTTL — время жизни FSM-сессии в Redis.
// Сбрасывается при каждом Transition или Update.
const defaultFSMTTL = 24 * time.Hour

// RawFSMState — сериализованное состояние для передачи через Store.
type RawFSMState struct {
	State string          `json:"state"`
	Data  json.RawMessage `json:"data,omitempty"`
}

// Store — интерфейс хранилища FSM-состояний.
type Store interface {
	Get(ctx context.Context, messenger, userID string) (*RawFSMState, error)
	Set(ctx context.Context, messenger, userID string, state *RawFSMState, ttl time.Duration) error
	Delete(ctx context.Context, messenger, userID string) error
}

// RedisStore — адаптер infraredis.FSMStore к интерфейсу Store.
type RedisStore struct {
	inner *infraredis.FSMStore
}

// NewRedisStore создаёт Store поверх Redis для использования в продакшене.
func NewRedisStore(s *infraredis.FSMStore) Store {
	return &RedisStore{inner: s}
}

func (r *RedisStore) Get(ctx context.Context, messenger, userID string) (*RawFSMState, error) {
	raw, err := r.inner.Get(ctx, messenger, userID)
	if err != nil {
		return nil, err
	}
	if raw == nil {
		return nil, nil
	}
	return &RawFSMState{State: raw.State, Data: raw.Data}, nil
}

func (r *RedisStore) Set(ctx context.Context, messenger, userID string, state *RawFSMState, ttl time.Duration) error {
	return r.inner.Set(ctx, messenger, userID, &infraredis.FSMState{
		State: state.State,
		Data:  state.Data,
	}, ttl)
}

func (r *RedisStore) Delete(ctx context.Context, messenger, userID string) error {
	return r.inner.Delete(ctx, messenger, userID)
}

// NewRegistry создаёт изолированный Registry без глобального состояния.
func NewRegistry() *Registry {
	return &Registry{
		transitions: make(map[State]map[State]TransitionRule),
		states:      make(map[State]string),
	}
}

// NewWithStore создаёт Machine с произвольной реализацией Store.
func NewWithStore(store Store, messenger, userID string, opts Options) Machine {
	reg := opts.Registry
	if reg == nil {
		reg = GlobalRegistry()
	}
	return &storeMachine{
		store:     store,
		messenger: messenger,
		userID:    userID,
		log:       opts.Log,
		tracer:    opts.Tracer,
		metrics:   opts.Metrics,
		registry:  reg,
	}
}

// storeMachine — реализация Machine поверх интерфейса Store.
type storeMachine struct {
	store     Store
	messenger string
	userID    string
	log       *core.Logger
	tracer    tracing.Tracer
	metrics   *metrics.AppMetrics
	registry  *Registry
}

func (m *storeMachine) Current(ctx context.Context) (*FSMState, error) {
	raw, err := m.store.Get(ctx, m.messenger, m.userID)
	if err != nil {
		return nil, err
	}
	if raw == nil {
		return NewIdleFSMState(), nil
	}
	return unmarshalRaw(raw)
}

func (m *storeMachine) Transition(ctx context.Context, newState State, data StateData) error {
	current, err := m.Current(ctx)
	if err != nil {
		return err
	}
	if !m.registry.IsAllowed(current.State, newState) {
		return ErrTransitionNotAllowed
	}
	raw, err := marshalRaw(&FSMState{State: newState, Data: data})
	if err != nil {
		return err
	}
	if err := m.store.Set(ctx, m.messenger, m.userID, raw, defaultFSMTTL); err != nil {
		return err
	}
	if m.metrics != nil {
		m.metrics.FSMTransitionsTotal.Inc(ctx,
			metrics.L(metrics.LabelState, string(newState)),
			metrics.L(metrics.LabelHandler, "transition"),
		)
	}
	return nil
}

func (m *storeMachine) Update(ctx context.Context, data StateData) error {
	current, err := m.Current(ctx)
	if err != nil {
		return err
	}
	current.Data = data
	raw, err := marshalRaw(current)
	if err != nil {
		return err
	}
	return m.store.Set(ctx, m.messenger, m.userID, raw, defaultFSMTTL)
}

func (m *storeMachine) Reset(ctx context.Context) error {
	return m.store.Delete(ctx, m.messenger, m.userID)
}

func (m *storeMachine) IsIdle(ctx context.Context) (bool, error) {
	current, err := m.Current(ctx)
	if err != nil {
		return false, err
	}
	return current.State.IsIdle(), nil
}

func (m *storeMachine) InState(ctx context.Context, state State) (bool, error) {
	current, err := m.Current(ctx)
	if err != nil {
		return false, err
	}
	return current.State == state, nil
}

func marshalRaw(s *FSMState) (*RawFSMState, error) {
	data, err := s.Data.MarshalJSON()
	if err != nil {
		return nil, err
	}
	return &RawFSMState{State: string(s.State), Data: data}, nil
}

func unmarshalRaw(raw *RawFSMState) (*FSMState, error) {
	s := &FSMState{State: State(raw.State), Data: NewStateData()}
	if len(raw.Data) > 0 {
		if err := s.Data.UnmarshalJSON(raw.Data); err != nil {
			return nil, err
		}
	}
	return s, nil
}
