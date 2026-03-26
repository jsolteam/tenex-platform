package fsm

import (
	"context"
	"errors"

	infraredis "github.com/jsolteam/tenex-platform/internal/infrastructure/redis"
	"github.com/jsolteam/tenex-platform/internal/platform/logger/core"
	"github.com/jsolteam/tenex-platform/internal/platform/observability/metrics"
	"github.com/jsolteam/tenex-platform/internal/platform/observability/tracing"
)

// ErrTransitionNotAllowed возвращается когда запрошенный переход
// не зарегистрирован в Registry.
var ErrTransitionNotAllowed = errors.New("fsm: переход не разрешён")

// Machine управляет FSM-состоянием одного пользователя.
// Один экземпляр создаётся на каждый входящий Update.
type Machine interface {
	// Current возвращает текущее состояние.
	// Если ключ не найден (первый визит или TTL истёк) — возвращает Idle.
	Current(ctx context.Context) (*FSMState, error)

	// Transition переводит в newState, заменяя данные на data.
	// Возвращает ErrTransitionNotAllowed если переход не зарегистрирован в Registry.
	Transition(ctx context.Context, newState State, data StateData) error

	// Update обновляет данные текущего шага без смены состояния.
	// Используется для накопления выборов в рамках одного шага
	// (например, пометка нескольких дней недели).
	Update(ctx context.Context, data StateData) error

	// Reset сбрасывает в Idle и удаляет все накопленные данные.
	Reset(ctx context.Context) error

	// IsIdle возвращает true если пользователь не в активном сценарии.
	IsIdle(ctx context.Context) (bool, error)

	// InState возвращает true если текущее состояние совпадает с state.
	InState(ctx context.Context, state State) (bool, error)
}

// Options — зависимости для observability.
type Options struct {
	Log      *core.Logger
	Tracer   tracing.Tracer
	Metrics  *metrics.AppMetrics
	Registry *Registry // nil = GlobalRegistry()
}

// New создаёт Machine для пользователя мессенджера поверх Redis.
//
//	machine := fsm.New(redisComp.FSMStore, "telegram", "123456", fsm.Options{
//	    Log: logger, Tracer: tracer, Metrics: appMetrics,
//	})
func New(store *infraredis.FSMStore, messenger, userID string, opts Options) Machine {
	return NewWithStore(NewRedisStore(store), messenger, userID, opts)
}
