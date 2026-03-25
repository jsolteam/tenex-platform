package redis

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jsolteam/tenex-platform/internal/infrastructure/infralog"
	goredis "github.com/redis/go-redis/v9"
	"go.opentelemetry.io/otel/attribute"
	"go.uber.org/zap"

	apperrors "github.com/jsolteam/tenex-platform/internal/platform/errors"
	"github.com/jsolteam/tenex-platform/internal/platform/observability/metrics"
)

const defaultFSMTTL = 24 * time.Hour

// FSMState — сериализуемое состояние FSM пользователя.
type FSMState struct {
	// State — имя текущего FSM-состояния
	State string `json:"state"`

	// Data — произвольный JSON payload, накапливаемый в течение сценария.
	Data json.RawMessage `json:"data,omitempty"`
}

// FSMStore — Redis-хранилище FSM-состояний пользователей.
//
// Ключ: fsm:{messenger}:{messengerUserID}
// TTL:  24 часа с момента последнего Set (сбрасывается при каждом обновлении)
type FSMStore struct {
	client *Client
	met    *metrics.RedisMetrics
}

// NewFSMStore создаёт FSMStore поверх существующего Client.
func NewFSMStore(c *Client) *FSMStore {
	return &FSMStore{client: c}
}

// Get возвращает текущее состояние пользователя.
// Если состояния нет (ключ истёк или не создавался), возвращает nil, nil.
func (s *FSMStore) Get(ctx context.Context, messenger, userID string) (*FSMState, error) {
	ctx, span := s.client.tracer.Start(ctx, "fsm.Get")
	defer span.End()
	start := time.Now()
	defer func() {
		s.met.RecordDuration(ctx, metrics.RedisComponentFSM, "Get", time.Since(start).Seconds())
	}()
	span.SetAttributes(
		attribute.String("fsm.messenger", messenger),
		attribute.String("fsm.user_id", userID),
	)

	key := fsmKey(messenger, userID)
	val, err := s.client.rdb.Get(ctx, key).Bytes()
	if errors.Is(err, goredis.Nil) {
		infralog.Debug(ctx, s.client.log, "fsm state not found",
			zap.String("key", key),
		)
		return nil, nil
	}
	if err != nil {
		appErr := apperrors.Redis("fsm.Get", err)
		infralog.Err(ctx, s.client.log, span, s.met,
			metrics.RedisComponentFSM, "Get", "fsm get failed", appErr,
			zap.String("key", key),
		)
		return nil, appErr
	}

	var state FSMState
	if err := json.Unmarshal(val, &state); err != nil {
		appErr := apperrors.Internal("fsm.Get.unmarshal", err)
		infralog.Err(ctx, s.client.log, span, s.met,
			metrics.RedisComponentFSM, "Get", "fsm state unmarshal failed", appErr,
			zap.String("key", key),
		)
		return nil, appErr
	}
	return &state, nil
}

// Set сохраняет состояние пользователя и сбрасывает TTL.
func (s *FSMStore) Set(ctx context.Context, messenger, userID string, state *FSMState, ttl time.Duration) error {
	ctx, span := s.client.tracer.Start(ctx, "fsm.Set")
	defer span.End()
	start := time.Now()
	defer func() {
		s.met.RecordDuration(ctx, metrics.RedisComponentFSM, "Set", time.Since(start).Seconds())
	}()

	if ttl <= 0 {
		ttl = defaultFSMTTL
	}

	key := fsmKey(messenger, userID)
	span.SetAttributes(
		attribute.String("fsm.key", key),
		attribute.String("fsm.state", state.State),
	)

	data, err := json.Marshal(state)
	if err != nil {
		appErr := apperrors.Internal("fsm.Set.marshal", err)
		infralog.Err(ctx, s.client.log, span, s.met,
			metrics.RedisComponentFSM, "Set", "fsm state marshal failed", appErr,
			zap.String("key", key),
		)
		return appErr
	}

	if err := s.client.rdb.Set(ctx, key, data, ttl).Err(); err != nil {
		appErr := apperrors.Redis("fsm.Set", err)
		infralog.Err(ctx, s.client.log, span, s.met,
			metrics.RedisComponentFSM, "Set", "fsm set failed", appErr,
			zap.String("key", key),
			zap.String("state", state.State),
		)
		return appErr
	}

	infralog.Debug(ctx, s.client.log, "fsm state updated",
		zap.String("key", key),
		zap.String("state", state.State),
	)
	return nil
}

// Delete удаляет состояние пользователя.
// Вызывается при завершении сценария или выходе в главное меню.
func (s *FSMStore) Delete(ctx context.Context, messenger, userID string) error {
	ctx, span := s.client.tracer.Start(ctx, "fsm.Delete")
	defer span.End()
	start := time.Now()
	defer func() {
		s.met.RecordDuration(ctx, metrics.RedisComponentFSM, "Delete", time.Since(start).Seconds())
	}()

	key := fsmKey(messenger, userID)
	span.SetAttributes(attribute.String("fsm.key", key))

	if err := s.client.rdb.Del(ctx, key).Err(); err != nil {
		appErr := apperrors.Redis("fsm.Delete", err)
		infralog.Err(ctx, s.client.log, span, s.met,
			metrics.RedisComponentFSM, "Delete", "fsm delete failed", appErr,
			zap.String("key", key),
		)
		return appErr
	}

	infralog.Debug(ctx, s.client.log, "fsm state deleted",
		zap.String("key", key),
	)
	return nil
}

// fsmKey формирует ключ: fsm:{messenger}:{userID}
func fsmKey(messenger, userID string) string {
	return "fsm:" + messenger + ":" + userID
}
