package redis_integration

import (
	"context"
	"testing"

	infraredis "github.com/jsolteam/tenex-platform/internal/infrastructure/redis"
	apperrors "github.com/jsolteam/tenex-platform/internal/platform/errors"
	"github.com/jsolteam/tenex-platform/internal/platform/logger/core"
	"github.com/jsolteam/tenex-platform/internal/platform/observability/tracing"
)

func TestClient_ConnectAndPing(t *testing.T) {
	client := setup(t)

	if err := client.Ping(context.Background()); err != nil {
		t.Fatalf("Ping after connect: %v", err)
	}
}

func TestClient_BadAddr_ReturnsRedisError(t *testing.T) {
	cfg := infraredis.Config{Addr: "localhost:19999"}
	_, err := infraredis.New(context.Background(), cfg, core.NewNoop(), tracing.NewNoop())
	if err == nil {
		t.Fatal("expected error for unreachable addr, got nil")
	}
	if apperrors.CodeOf(err) != apperrors.ErrRedisUnavailable {
		t.Errorf("CodeOf = %q, want ErrRedisUnavailable", apperrors.CodeOf(err))
	}
}

func TestClient_Close_Idempotent(t *testing.T) {
	client := setup(t)

	// Первый Close должен быть успешным
	if err := client.Close(); err != nil {
		t.Fatalf("first Close: %v", err)
	}
	// Cleanup в setup тоже вызовет Close — не должен паниковать
}
