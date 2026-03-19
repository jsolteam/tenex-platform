// Package repolog предоставляет хелперы для логирования, трейсинга и метрик
//
// Использование в репозиториях:
//
//	start := time.Now()
//	ctx, span := r.tracer.Start(ctx, "userrepo.GetByID")
//	defer span.End()
//	defer r.met.RecordDuration(ctx, "user", "GetByID", time.Since(start).Seconds())
//
//	if err != nil {
//	    appErr := apperrors.DB("userrepo.GetByID", err)
//	    repolog.Err(ctx, r.log, span, r.met, "user", "GetByID", appErr, zap.Int64("id", id))
//	    return nil, appErr
//	}
package repolog

import (
	"context"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.uber.org/zap"

	apperrors "github.com/jsolteam/tenex-platform/internal/platform/errors"
	contextlog "github.com/jsolteam/tenex-platform/internal/platform/logger/context"
	"github.com/jsolteam/tenex-platform/internal/platform/logger/core"
	"github.com/jsolteam/tenex-platform/internal/platform/observability/metrics"
	"github.com/jsolteam/tenex-platform/internal/platform/observability/tracing"
)

func Err(
	ctx context.Context,
	l *core.Logger,
	span tracing.Span,
	met *metrics.DBMetrics,
	repo, method string,
	msg string,
	appErr *apperrors.AppError,
	extra ...zap.Field,
) {
	// ── Спан ──────────────────────────────────────────────────────────────
	span.RecordError(appErr)
	span.SetStatus(codes.Error, string(appErr.Code))
	span.SetAttributes(
		attribute.String("error.code", string(appErr.Code)),
		attribute.String("error.module", appErr.Module),
	)

	// ── Метрики ───────────────────────────────────────────────────────────
	met.RecordError(ctx, repo, method, string(appErr.Code))

	// ── Лог ───────────────────────────────────────────────────────────────
	logFields := buildFields(appErr, extra...)
	cl := contextlog.FromCtx(ctx, l)

	switch appErr.Severity {
	case apperrors.SeverityFatal, apperrors.SeverityError:
		cl.Error(msg, logFields...)
	case apperrors.SeverityWarn:
		cl.Warn(msg, logFields...)
	case apperrors.SeverityInfo:
		cl.Info(msg, logFields...)
	default:
		cl.Debug(msg, logFields...)
	}
}

// Debug логирует штатный miss (ErrNotFound, RowsAffected == 0) на уровне Debug.
// Спан остаётся успешным, метрики ошибок не трогаются.
func Debug(ctx context.Context, l *core.Logger, msg string, extra ...zap.Field) {
	contextlog.FromCtx(ctx, l).Debug(msg, extra...)
}

// buildFields собирает []zap.Field: zap.Error + атрибуты AppError + extra.
func buildFields(appErr *apperrors.AppError, extra ...zap.Field) []zap.Field {
	out := make([]zap.Field, 0, 1+len(appErr.Attrs())+len(extra))
	out = append(out, zap.Error(appErr))
	for _, a := range appErr.Attrs() {
		out = append(out, zap.Any(a.Key, a.Val))
	}
	return append(out, extra...)
}
