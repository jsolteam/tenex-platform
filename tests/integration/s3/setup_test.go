package s3_integration

import (
	"context"
	"os"
	"strconv"
	"testing"
	"time"

	infras3 "github.com/jsolteam/tenex-platform/internal/infrastructure/s3"
	"github.com/jsolteam/tenex-platform/internal/platform/logger/core"
	"github.com/jsolteam/tenex-platform/internal/platform/observability/tracing"
)

func setupS3(t *testing.T) *infras3.Client {
	t.Helper()

	endpoint := os.Getenv("S3_ENDPOINT")
	if endpoint == "" {
		t.Skip("S3_ENDPOINT not set — skipping s3 integration test")
	}

	cfg := infras3.Config{
		Endpoint: endpoint,
		Key:      os.Getenv("S3_KEY"),
		Secret:   os.Getenv("S3_SECRET"),
		Bucket:   os.Getenv("S3_BUCKET"),
		UseSSL:   envBool("S3_USE_SSL", false),
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	client, err := infras3.New(ctx, cfg, core.NewNoop(), tracing.NewNoop())
	if err != nil {
		t.Fatalf("s3 setup: %v", err)
	}

	t.Cleanup(func() {
		_ = client.Close()
	})

	return client
}

func envBool(key string, fallback bool) bool {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	parsed, err := strconv.ParseBool(v)
	if err != nil {
		return fallback
	}
	return parsed
}
