package s3

import (
	"context"
	"errors"
	"io"
	"net/url"
	"time"

	"github.com/minio/minio-go/v7"
	"go.uber.org/zap"

	"github.com/jsolteam/tenex-platform/internal/infrastructure/infralog"
	apperrors "github.com/jsolteam/tenex-platform/internal/platform/errors"
)

func (c *Client) Upload(ctx context.Context, key string, reader io.Reader, size int64, contentType string) error {
	ctx, span := c.tracer.Start(ctx, "s3.Upload")
	defer span.End()
	start := time.Now()
	defer func() {
		c.met.RecordDuration(ctx, "s3", "Upload", time.Since(start).Seconds())
	}()

	_, err := c.mc.PutObject(ctx, c.bucket, key, reader, size, minio.PutObjectOptions{
		ContentType: contentType,
	})
	if err != nil {
		appErr := apperrors.S3("s3.Upload", err)
		infralog.Err(ctx, c.log, span, c.met, "s3", "Upload", "s3 upload failed", appErr,
			zap.String("bucket", c.bucket),
			zap.String("key", key),
		)
		return appErr
	}

	return nil
}

func (c *Client) Download(ctx context.Context, key string) (io.ReadCloser, error) {
	ctx, span := c.tracer.Start(ctx, "s3.Download")
	defer span.End()
	start := time.Now()
	defer func() {
		c.met.RecordDuration(ctx, "s3", "Download", time.Since(start).Seconds())
	}()

	obj, err := c.mc.GetObject(ctx, c.bucket, key, minio.GetObjectOptions{})
	if err != nil {
		appErr := apperrors.S3("s3.Download", err)
		infralog.Err(ctx, c.log, span, c.met, "s3", "Download", "s3 get object failed", appErr,
			zap.String("bucket", c.bucket),
			zap.String("key", key),
		)
		return nil, appErr
	}

	if _, err := obj.Stat(); err != nil {
		_ = obj.Close()
		appErr := apperrors.S3("s3.Download.stat", err)
		infralog.Err(ctx, c.log, span, c.met, "s3", "Download", "s3 get object stat failed", appErr,
			zap.String("bucket", c.bucket),
			zap.String("key", key),
		)
		return nil, appErr
	}

	return obj, nil
}

func (c *Client) Delete(ctx context.Context, key string) error {
	ctx, span := c.tracer.Start(ctx, "s3.Delete")
	defer span.End()
	start := time.Now()
	defer func() {
		c.met.RecordDuration(ctx, "s3", "Delete", time.Since(start).Seconds())
	}()

	err := c.mc.RemoveObject(ctx, c.bucket, key, minio.RemoveObjectOptions{})
	if err != nil {
		var respErr minio.ErrorResponse
		if errors.As(err, &respErr) && (respErr.Code == "NoSuchKey" || respErr.Code == "NoSuchObject") {
			return nil
		}
		appErr := apperrors.S3("s3.Delete", err)
		infralog.Err(ctx, c.log, span, c.met, "s3", "Delete", "s3 delete object failed", appErr,
			zap.String("bucket", c.bucket),
			zap.String("key", key),
		)
		return appErr
	}

	return nil
}

func (c *Client) PresignedURL(ctx context.Context, key string, expires time.Duration) (string, error) {
	ctx, span := c.tracer.Start(ctx, "s3.PresignedURL")
	defer span.End()
	start := time.Now()
	defer func() {
		c.met.RecordDuration(ctx, "s3", "PresignedURL", time.Since(start).Seconds())
	}()

	u, err := c.mc.PresignedGetObject(ctx, c.bucket, key, expires, url.Values{})
	if err != nil {
		appErr := apperrors.S3("s3.PresignedURL", err)
		infralog.Err(ctx, c.log, span, c.met, "s3", "PresignedURL", "s3 presigned url failed", appErr,
			zap.String("bucket", c.bucket),
			zap.String("key", key),
		)
		return "", appErr
	}
	return u.String(), nil
}
