package userrepo

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"go.uber.org/zap"

	"github.com/jsolteam/tenex-platform/internal/domain/user"
	"github.com/jsolteam/tenex-platform/internal/infrastructure/repolog"
	apperrors "github.com/jsolteam/tenex-platform/internal/platform/errors"
	"github.com/jsolteam/tenex-platform/internal/platform/logger/core"
	"github.com/jsolteam/tenex-platform/internal/platform/observability/metrics"
	"github.com/jsolteam/tenex-platform/internal/platform/observability/tracing"
)

const statsRepoName = "user_statistics"

type StatisticsRepository struct {
	db     *sql.DB
	log    *core.Logger
	tracer tracing.Tracer
	met    *metrics.DBMetrics
}

func NewStatistics(db *sql.DB, log *core.Logger, tracer tracing.Tracer, met *metrics.DBMetrics) *StatisticsRepository {
	return &StatisticsRepository{
		db:     db,
		log:    log.With(zap.String("repo", statsRepoName)),
		tracer: tracer,
		met:    met,
	}
}

func (r *StatisticsRepository) GetByUserID(ctx context.Context, userID int64) (*user.UserStatistics, error) {
	const method = "GetByUserID"
	start := time.Now()
	ctx, span := r.tracer.Start(ctx, "userrepo.stats."+method)
	defer span.End()
	defer func() { r.met.RecordDuration(ctx, statsRepoName, method, time.Since(start).Seconds()) }()

	const q = `SELECT user_id, total_reminders, confirmed, skipped, adherence_rate, updated_at FROM user_statistics WHERE user_id = $1`

	s := &user.UserStatistics{}
	err := r.db.QueryRowContext(ctx, q, userID).Scan(
		&s.UserID, &s.TotalReminders, &s.Confirmed, &s.Skipped, &s.AdherenceRate, &s.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		repolog.Debug(ctx, r.log, "user statistics not found", zap.Int64("user_id", userID))
		return nil, user.ErrNotFound
	}
	if err != nil {
		appErr := apperrors.DB("userrepo.stats."+method, err)
		repolog.Err(ctx, r.log, span, r.met, statsRepoName, method, "failed to get user statistics", appErr, zap.Int64("user_id", userID))
		return nil, appErr
	}
	return s, nil
}

func (r *StatisticsRepository) Upsert(ctx context.Context, s *user.UserStatistics) error {
	const method = "Upsert"
	start := time.Now()
	ctx, span := r.tracer.Start(ctx, "userrepo.stats."+method)
	defer span.End()
	defer func() { r.met.RecordDuration(ctx, statsRepoName, method, time.Since(start).Seconds()) }()

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
		appErr := apperrors.DB("userrepo.stats."+method, err)
		repolog.Err(ctx, r.log, span, r.met, statsRepoName, method, "failed to upsert user statistics", appErr, zap.Int64("user_id", s.UserID))
		return appErr
	}
	return nil
}
