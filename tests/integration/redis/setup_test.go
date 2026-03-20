package redis_integration

import (
	"context"
	"os"
	"testing"
	"time"

	infraredis "github.com/jsolteam/tenex-platform/internal/infrastructure/redis"
	"github.com/jsolteam/tenex-platform/internal/platform/logger/core"
	"github.com/jsolteam/tenex-platform/internal/platform/observability/tracing"
)

func setup(t *testing.T) *infraredis.Client {
	t.Helper()

	addr := os.Getenv("REDIS_ADDR")
	if addr == "" {
		t.Skip("REDIS_ADDR not set — skipping redis integration test")
	}

	cfg := infraredis.Config{
		Addr:     addr,
		Password: os.Getenv("REDIS_PASSWORD"),
		DB:       14, // отдельная DB для тестов, не трогает основную
	}

	ctx := context.Background()
	client, err := infraredis.New(ctx, cfg, core.NewNoop(), tracing.NewNoop())
	if err != nil {
		t.Fatalf("redis setup: connect: %v", err)
	}

	// Чистим DB перед тестом для полной изоляции
	if err := client.Underlying().FlushDB(ctx).Err(); err != nil {
		t.Fatalf("redis setup: flushdb: %v", err)
	}

	t.Cleanup(func() {
		_ = client.Underlying().FlushDB(context.Background())
		_ = client.Close()
	})

	return client
}

// key генерирует уникальный ключ на основе имени теста и времени.
// Предотвращает пересечения между параллельными тестами.
func key(t *testing.T, suffix string) string {
	t.Helper()
	return t.Name() + ":" + suffix + ":" + time.Now().Format("150405000000000")
}
