package userrepo

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/jsolteam/tenex-platform/internal/domain/user"
)

type StatisticsRepository struct {
	db *sql.DB
}

func NewStatistics(db *sql.DB) *StatisticsRepository {
	return &StatisticsRepository{db: db}
}

// GetByUserID возвращает статистику пользователя.
func (r *StatisticsRepository) GetByUserID(ctx context.Context, userID int64) (*user.UserStatistics, error) {
	const q = `
		SELECT user_id, total_reminders, confirmed, skipped, adherence_rate, updated_at
		FROM user_statistics
		WHERE user_id = $1`

	s := &user.UserStatistics{}
	err := r.db.QueryRowContext(ctx, q, userID).Scan(
		&s.UserID, &s.TotalReminders, &s.Confirmed, &s.Skipped, &s.AdherenceRate, &s.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, user.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("userrepo.GetStatsByUserID: %w", err)
	}
	return s, nil
}

// Upsert создаёт или обновляет статистику пользователя.
func (r *StatisticsRepository) Upsert(ctx context.Context, s *user.UserStatistics) error {
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
		return fmt.Errorf("userrepo.UpsertStats: %w", err)
	}
	return nil
}
