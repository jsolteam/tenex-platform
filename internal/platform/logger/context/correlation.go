package contextlog

import (
	"context"

	"go.uber.org/zap"
)

type contextKey struct{ name string }

var (
	keyRequestID = &contextKey{"request_id"}
	keyUserID    = &contextKey{"user_id"}
	keySessionID = &contextKey{"session_id"}
)

func WithRequestID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, keyRequestID, id)
}

func WithUserID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, keyUserID, id)
}

func WithSessionID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, keySessionID, id)
}

func Extract(ctx context.Context) []zap.Field {
	var f []zap.Field
	if v, _ := ctx.Value(keyRequestID).(string); v != "" {
		f = append(f, zap.String("request_id", v))
	}
	if v, _ := ctx.Value(keyUserID).(string); v != "" {
		f = append(f, zap.String("user_id", v))
	}
	if v, _ := ctx.Value(keySessionID).(string); v != "" {
		f = append(f, zap.String("session_id", v))
	}
	return f
}
