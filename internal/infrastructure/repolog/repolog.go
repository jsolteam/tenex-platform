package repolog

import (
	"context"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.uber.org/zap"

	apperrors "github.com/jsolteam/tenex-platform/internal/platform/errors"
	contextlog "github.com/jsolteam/tenex-platform/internal/platform/logger/context"
	"github.com/jsolteam/tenex-platform/internal/platform/logger/core"
	"github.com/jsolteam/tenex-platform/internal/platform/observability/tracing"
)

func Err(
	ctx context.Context,
	l *core.Logger,
	span tracing.Span,
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

// Debug логирует штатный miss (ErrNotFound, not-found on update/delete) на уровне Debug.
// Спан остаётся успешным — это не инфраструктурная ошибка.
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
