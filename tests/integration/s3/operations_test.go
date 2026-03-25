package s3_integration

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	infras3 "github.com/jsolteam/tenex-platform/internal/infrastructure/s3"
	apperrors "github.com/jsolteam/tenex-platform/internal/platform/errors"
	"github.com/jsolteam/tenex-platform/internal/platform/logger/core"
	"github.com/jsolteam/tenex-platform/internal/platform/observability/tracing"
)

func TestS3_UploadDownloadDelete_Roundtrip(t *testing.T) {
	client := setupS3(t)
	ctx := context.Background()

	key := "integration/" + uuid.NewString() + ".txt"
	payload := []byte("tenex-s3-roundtrip")

	if err := client.Upload(ctx, key, bytes.NewReader(payload), int64(len(payload)), "text/plain"); err != nil {
		t.Fatalf("Upload: %v", err)
	}

	r, err := client.Download(ctx, key)
	if err != nil {
		t.Fatalf("Download: %v", err)
	}
	defer r.Close()

	got, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	if !bytes.Equal(got, payload) {
		t.Fatalf("payload mismatch: got %q want %q", string(got), string(payload))
	}

	if err := client.Delete(ctx, key); err != nil {
		t.Fatalf("Delete: %v", err)
	}
}

func TestS3_PresignedURL_Returns200(t *testing.T) {
	client := setupS3(t)
	ctx := context.Background()

	key := "integration/" + uuid.NewString() + ".txt"
	payload := []byte("tenex-s3-presigned")

	if err := client.Upload(ctx, key, bytes.NewReader(payload), int64(len(payload)), "text/plain"); err != nil {
		t.Fatalf("Upload: %v", err)
	}
	defer func() { _ = client.Delete(ctx, key) }()

	u, err := client.PresignedURL(ctx, key, 5*time.Minute)
	if err != nil {
		t.Fatalf("PresignedURL: %v", err)
	}

	resp, err := http.Get(u) //nolint:gosec
	if err != nil {
		t.Fatalf("GET presigned: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
}

func TestS3_New_NonExistingBucket_ReturnsErrS3(t *testing.T) {
	endpoint := getenvOrSkip(t, "S3_ENDPOINT")

	cfg := infras3.Config{
		Endpoint: endpoint,
		Key:      getenvOrSkip(t, "S3_KEY"),
		Secret:   getenvOrSkip(t, "S3_SECRET"),
		Bucket:   "tenex-missing-" + uuid.NewString(),
		UseSSL:   envBool("S3_USE_SSL", false),
	}

	_, err := infras3.New(context.Background(), cfg, core.NewNoop(), tracing.NewNoop())
	if err == nil {
		t.Fatal("expected error for non-existing bucket, got nil")
	}
	if apperrors.CodeOf(err) != apperrors.ErrS3 {
		t.Fatalf("error code = %q, want %q", apperrors.CodeOf(err), apperrors.ErrS3)
	}
}

func getenvOrSkip(t *testing.T, k string) string {
	t.Helper()
	v := os.Getenv(k)
	if v == "" {
		t.Skipf("%s not set — skipping s3 integration test", k)
	}
	return v
}
