package userrepo

import (
	"context"
	"database/sql"
	"errors"

	"go.uber.org/zap"

	"github.com/jsolteam/tenex-platform/internal/domain/user"
	"github.com/jsolteam/tenex-platform/internal/infrastructure/repolog"
	apperrors "github.com/jsolteam/tenex-platform/internal/platform/errors"
	"github.com/jsolteam/tenex-platform/internal/platform/logger/core"
	"github.com/jsolteam/tenex-platform/internal/platform/observability/tracing"
)

type StatisticsRepository struct {
	db     *sql.DB
	log    *core.Logger
	tracer tracing.Tracer
}

func NewStatistics(db *sql.DB, log *core.Logger, tracer tracing.Tracer) *StatisticsRepository {
	return &StatisticsRepository{
		db:     db,
		log:    log.With(zap.String("repo", "user_statistics")),
		tracer: tracer,
	}
}

func (r *StatisticsRepository) GetByUserID(ctx context.Context, userID int64) (*user.UserStatistics, error) {
	ctx, span := r.tracer.Start(ctx, "userrepo.GetStatsByUserID")
	defer span.End()

	const q = `
		SELECT user_id, total_reminders, confirmed, skipped, adherence_rate, updated_at
		FROM user_statistics WHERE user_id = $1`

	s := &user.UserStatistics{}
	err := r.db.QueryRowContext(ctx, q, userID).Scan(
		&s.UserID, &s.TotalReminders, &s.Confirmed, &s.Skipped, &s.AdherenceRate, &s.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		repolog.Debug(ctx, r.log, "user statistics not found", zap.Int64("user_id", userID))
		return nil, user.ErrNotFound
	}
	if err != nil {
		appErr := apperrors.DB("userrepo.GetStatsByUserID", err)
		repolog.Err(ctx, r.log, span, "failed to get user statistics", appErr, zap.Int64("user_id", userID))
		return nil, appErr
	}
	return s, nil
}

func (r *StatisticsRepository) Upsert(ctx context.Context, s *user.UserStatistics) error {
	ctx, span := r.tracer.Start(ctx, "userrepo.UpsertStats")
	defer span.End()

	const q = `
		INSERT INTO user_statistics (user_id, total_reminders, confirmed, skipped, adherence_rate, updated_at)
		VALUES ($1, $2, $3, $4, $5, now())
		ON CONFLICT (user_id) DO UPDATE SET
			total_reminders = EXCLUDED.total_reminders,
			confirmed       = EXCLUDED.confirmed,
			skipped         = EXCLUDED.skipped,
			adherence_rate  = EXCLUDED.adherence_rate,
			updated_at      = now()
		RETURNING updated_at`

	if s.TotalReminders > 0 {
		s.AdherenceRate = float64(s.Confirmed) / float64(s.TotalReminders)
	}

	err := r.db.QueryRowContext(ctx, q,
		s.UserID, s.TotalReminders, s.Confirmed, s.Skipped, s.AdherenceRate,
	).Scan(&s.UpdatedAt)
	if err != nil {
		appErr := apperrors.DB("userrepo.UpsertStats", err)
		repolog.Err(ctx, r.log, span, "failed to upsert user statistics", appErr, zap.Int64("user_id", s.UserID))
		return appErr
	}
	return nil
}
