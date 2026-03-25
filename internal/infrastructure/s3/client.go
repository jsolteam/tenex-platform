package s3

import (
	"context"
	"fmt"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"go.uber.org/zap"

	"github.com/jsolteam/tenex-platform/internal/infrastructure/infralog"
	apperrors "github.com/jsolteam/tenex-platform/internal/platform/errors"
	"github.com/jsolteam/tenex-platform/internal/platform/logger/core"
	"github.com/jsolteam/tenex-platform/internal/platform/observability/metrics"
	"github.com/jsolteam/tenex-platform/internal/platform/observability/tracing"
)

type Config struct {
	Endpoint string
	Key      string
	Secret   string
	Bucket   string
	UseSSL   bool
}

type Client struct {
	mc     *minio.Client
	log    *core.Logger
	tracer tracing.Tracer
	met    *metrics.S3Metrics
	bucket string
}

func New(
	ctx context.Context,
	cfg Config,
	log *core.Logger,
	tracer tracing.Tracer,
	metOpt ...*metrics.S3Metrics,
) (*Client, error) {
	var met *metrics.S3Metrics
	if len(metOpt) > 0 {
		met = metOpt[0]
	}
	if met == nil {
		met, _ = metrics.NewS3Metrics(metrics.NewNoop())
	}

	mc, err := minio.New(cfg.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.Key, cfg.Secret, ""),
		Secure: cfg.UseSSL,
	})
	if err != nil {
		return nil, apperrors.S3("s3.New.client", err)
	}

	client := &Client{
		mc:     mc,
		log:    log.With(zap.String("component", "s3"), zap.String("endpoint", cfg.Endpoint), zap.String("bucket", cfg.Bucket)),
		tracer: tracer,
		met:    met,
		bucket: cfg.Bucket,
	}

	if err := client.Ping(ctx); err != nil {
		return nil, err
	}

	return client, nil
}

func (c *Client) Ping(ctx context.Context) error {
	ctx, span := c.tracer.Start(ctx, "s3.Ping")
	defer span.End()
	start := time.Now()
	defer func() {
		c.met.RecordDuration(ctx, "s3", "Ping", time.Since(start).Seconds())
	}()

	exists, err := c.mc.BucketExists(ctx, c.bucket)
	if err != nil {
		appErr := apperrors.S3("s3.Ping.bucket_exists", err)
		infralog.Err(ctx, c.log, span, c.met, "s3", "Ping", "s3 bucket exists check failed", appErr,
			zap.String("bucket", c.bucket),
		)
		return appErr
	}
	if !exists {
		appErr := apperrors.S3("s3.Ping.bucket_exists", fmt.Errorf("bucket %q does not exist", c.bucket))
		infralog.Err(ctx, c.log, span, c.met, "s3", "Ping", "s3 bucket does not exist", appErr,
			zap.String("bucket", c.bucket),
		)
		return appErr
	}
	return nil
}

func (c *Client) Close() error {
	return nil
}
