package redis

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	goredis "github.com/redis/go-redis/v9"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.uber.org/zap"

	apperrors "github.com/jsolteam/tenex-platform/internal/platform/errors"
	contextlog "github.com/jsolteam/tenex-platform/internal/platform/logger/context"
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
	span.SetAttributes(
		attribute.String("fsm.messenger", messenger),
		attribute.String("fsm.user_id", userID),
	)

	key := fsmKey(messenger, userID)
	val, err := s.client.rdb.Get(ctx, key).Bytes()
	if errors.Is(err, goredis.Nil) {
		contextlog.FromCtx(ctx, s.client.log).Debug("fsm state not found",
			zap.String("key", key),
		)
		return nil, nil
	}
	if err != nil {
		appErr := apperrors.Redis("fsm.Get", err)
		span.RecordError(appErr)
		span.SetStatus(codes.Error, string(appErr.Code))
		contextlog.FromCtx(ctx, s.client.log).Error("fsm get failed",
			zap.Error(appErr),
			zap.String("key", key),
		)
		return nil, appErr
	}

	var state FSMState
	if err := json.Unmarshal(val, &state); err != nil {
		appErr := apperrors.Internal("fsm.Get.unmarshal", err)
		contextlog.FromCtx(ctx, s.client.log).Error("fsm state unmarshal failed",
			zap.Error(appErr),
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
		contextlog.FromCtx(ctx, s.client.log).Error("fsm state marshal failed",
			zap.Error(appErr),
			zap.String("key", key),
		)
		return appErr
	}

	if err := s.client.rdb.Set(ctx, key, data, ttl).Err(); err != nil {
		appErr := apperrors.Redis("fsm.Set", err)
		span.RecordError(appErr)
		span.SetStatus(codes.Error, string(appErr.Code))
		contextlog.FromCtx(ctx, s.client.log).Error("fsm set failed",
			zap.Error(appErr),
			zap.String("key", key),
			zap.String("state", state.State),
		)
		return appErr
	}

	contextlog.FromCtx(ctx, s.client.log).Debug("fsm state updated",
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

	key := fsmKey(messenger, userID)
	span.SetAttributes(attribute.String("fsm.key", key))

	if err := s.client.rdb.Del(ctx, key).Err(); err != nil {
		appErr := apperrors.Redis("fsm.Delete", err)
		span.RecordError(appErr)
		span.SetStatus(codes.Error, string(appErr.Code))
		contextlog.FromCtx(ctx, s.client.log).Error("fsm delete failed",
			zap.Error(appErr),
			zap.String("key", key),
		)
		return appErr
	}

	contextlog.FromCtx(ctx, s.client.log).Debug("fsm state deleted",
		zap.String("key", key),
	)
	return nil
}

// fsmKey формирует ключ: fsm:{messenger}:{userID}
func fsmKey(messenger, userID string) string {
	return "fsm:" + messenger + ":" + userID
}
