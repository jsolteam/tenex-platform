package reminderrepo

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/jsolteam/tenex-platform/internal/domain/reminder"
	apperrors "github.com/jsolteam/tenex-platform/internal/platform/errors"
)

type Repository struct {
	db *sql.DB
}

func New(db *sql.DB) *Repository {
	return &Repository{db: db}
}

// GetByID возвращает напоминание по ID и scheduled_at.
func (r *Repository) GetByID(ctx context.Context, id int64, scheduledAt time.Time) (*reminder.Reminder, error) {
	const q = `
		SELECT id, user_id, medicine_id, schedule_id, scheduled_at,
		       status, retry_count, postpone_count, idempotency_key, created_at
		FROM reminders
		WHERE id = $1 AND scheduled_at = $2`

	rem, err := scanReminder(r.db.QueryRowContext(ctx, q, id, scheduledAt))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, reminder.ErrNotFound
	}
	if err != nil {
		return nil, apperrors.DB("reminderrepo.GetByID", err)
	}
	return rem, nil
}

// GetByIdempotencyKey возвращает напоминание по ключу идемпотентности.
func (r *Repository) GetByIdempotencyKey(ctx context.Context, key uuid.UUID) (*reminder.Reminder, error) {
	const q = `
		SELECT id, user_id, medicine_id, schedule_id, scheduled_at,
		       status, retry_count, postpone_count, idempotency_key, created_at
		FROM reminders
		WHERE idempotency_key = $1`

	rem, err := scanReminder(r.db.QueryRowContext(ctx, q, key))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, reminder.ErrNotFound
	}
	if err != nil {
		return nil, apperrors.DB("reminderrepo.GetByIdempotencyKey", err)
	}
	return rem, nil
}

// ListByUser возвращает историю напоминаний пользователя за период.
func (r *Repository) ListByUser(ctx context.Context, userID int64, from, to time.Time) ([]reminder.Reminder, error) {
	const q = `
		SELECT id, user_id, medicine_id, schedule_id, scheduled_at,
		       status, retry_count, postpone_count, idempotency_key, created_at
		FROM reminders
		WHERE user_id = $1 AND scheduled_at BETWEEN $2 AND $3
		ORDER BY scheduled_at DESC`

	result, err := queryList(ctx, r.db, q, userID, from, to)
	if err != nil {
		return nil, apperrors.DB("reminderrepo.ListByUser", err)
	}
	return result, nil
}

// ListPending возвращает напоминания требующие обработки.
func (r *Repository) ListPending(ctx context.Context, before time.Time, limit int) ([]reminder.Reminder, error) {
	const q = `
		SELECT id, user_id, medicine_id, schedule_id, scheduled_at,
		       status, retry_count, postpone_count, idempotency_key, created_at
		FROM reminders
		WHERE status IN ('pending', 'sent') AND scheduled_at <= $1
		ORDER BY scheduled_at ASC
		LIMIT $2`

	result, err := queryList(ctx, r.db, q, before, limit)
	if err != nil {
		return nil, apperrors.DB("reminderrepo.ListPending", err)
	}
	return result, nil
}

// ListBySchedule возвращает напоминания для расписания за период.
func (r *Repository) ListBySchedule(ctx context.Context, scheduleID int64, from, to time.Time) ([]reminder.Reminder, error) {
	const q = `
		SELECT id, user_id, medicine_id, schedule_id, scheduled_at,
		       status, retry_count, postpone_count, idempotency_key, created_at
		FROM reminders
		WHERE schedule_id = $1 AND scheduled_at BETWEEN $2 AND $3
		ORDER BY scheduled_at ASC`

	result, err := queryList(ctx, r.db, q, scheduleID, from, to)
	if err != nil {
		return nil, apperrors.DB("reminderrepo.ListBySchedule", err)
	}
	return result, nil
}

// Create создаёт новое напоминание.
func (r *Repository) Create(ctx context.Context, rem *reminder.Reminder) error {
	const q = `
		INSERT INTO reminders (user_id, medicine_id, schedule_id, scheduled_at, status, idempotency_key)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, created_at`

	err := r.db.QueryRowContext(ctx, q,
		rem.UserID, rem.MedicineID, rem.ScheduleID,
		rem.ScheduledAt, rem.Status, rem.IdempotencyKey,
	).Scan(&rem.ID, &rem.CreatedAt)
	if err != nil {
		return apperrors.DB("reminderrepo.Create", err)
	}
	return nil
}

// UpdateStatus обновляет статус напоминания.
func (r *Repository) UpdateStatus(ctx context.Context, id int64, scheduledAt time.Time, status reminder.Status) error {
	const q = `
		UPDATE reminders SET status = $1
		WHERE id = $2 AND scheduled_at = $3`

	res, err := r.db.ExecContext(ctx, q, status, id, scheduledAt)
	if err != nil {
		return apperrors.DB("reminderrepo.UpdateStatus", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return reminder.ErrNotFound
	}
	return nil
}

// IncrementRetry увеличивает счётчик ретраев.
func (r *Repository) IncrementRetry(ctx context.Context, id int64, scheduledAt time.Time) error {
	const q = `
		UPDATE reminders SET retry_count = retry_count + 1
		WHERE id = $1 AND scheduled_at = $2`

	res, err := r.db.ExecContext(ctx, q, id, scheduledAt)
	if err != nil {
		return apperrors.DB("reminderrepo.IncrementRetry", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return reminder.ErrNotFound
	}
	return nil
}

// IncrementPostpone увеличивает счётчик откладываний.
func (r *Repository) IncrementPostpone(ctx context.Context, id int64, scheduledAt time.Time) error {
	const q = `
		UPDATE reminders SET postpone_count = postpone_count + 1
		WHERE id = $1 AND scheduled_at = $2`

	res, err := r.db.ExecContext(ctx, q, id, scheduledAt)
	if err != nil {
		return apperrors.DB("reminderrepo.IncrementPostpone", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return reminder.ErrNotFound
	}
	return nil
}

// ── helpers ───────────────────────────────────────────────────────────────

type scanner interface {
	Scan(dest ...any) error
}

func scanReminder(s scanner) (*reminder.Reminder, error) {
	rem := &reminder.Reminder{}
	err := s.Scan(
		&rem.ID, &rem.UserID, &rem.MedicineID, &rem.ScheduleID,
		&rem.ScheduledAt, &rem.Status, &rem.RetryCount,
		&rem.PostponeCount, &rem.IdempotencyKey, &rem.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	return rem, nil
}

func queryList(ctx context.Context, db *sql.DB, q string, args ...any) ([]reminder.Reminder, error) {
	rows, err := db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []reminder.Reminder
	for rows.Next() {
		rem, err := scanReminder(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, *rem)
	}
	return result, rows.Err()
}
