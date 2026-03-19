package reminderrepo

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/jsolteam/tenex-platform/internal/domain/reminder"
	"github.com/jsolteam/tenex-platform/internal/infrastructure/repolog"
	apperrors "github.com/jsolteam/tenex-platform/internal/platform/errors"
	"github.com/jsolteam/tenex-platform/internal/platform/logger/core"
	"github.com/jsolteam/tenex-platform/internal/platform/observability/metrics"
	"github.com/jsolteam/tenex-platform/internal/platform/observability/tracing"
)

const repoName = "reminder"

type Repository struct {
	db     *sql.DB
	log    *core.Logger
	tracer tracing.Tracer
	met    *metrics.DBMetrics
}

func New(db *sql.DB, log *core.Logger, tracer tracing.Tracer, met *metrics.DBMetrics) *Repository {
	return &Repository{
		db:     db,
		log:    log.With(zap.String("repo", repoName)),
		tracer: tracer,
		met:    met,
	}
}

func (r *Repository) GetByID(ctx context.Context, id int64, scheduledAt time.Time) (*reminder.Reminder, error) {
	const method = "GetByID"
	start := time.Now()
	ctx, span := r.tracer.Start(ctx, "reminderrepo."+method)
	defer span.End()
	defer func() { r.met.RecordDuration(ctx, repoName, method, time.Since(start).Seconds()) }()

	rem, err := scanReminder(r.db.QueryRowContext(ctx, `SELECT id, user_id, medicine_id, schedule_id, scheduled_at, status, retry_count, postpone_count, idempotency_key, created_at FROM reminders WHERE id=$1 AND scheduled_at=$2`, id, scheduledAt))
	if errors.Is(err, sql.ErrNoRows) {
		repolog.Debug(ctx, r.log, "reminder not found", zap.Int64("id", id))
		return nil, reminder.ErrNotFound
	}
	if err != nil {
		appErr := apperrors.DB("reminderrepo."+method, err)
		repolog.Err(ctx, r.log, span, r.met, repoName, method, "failed to get reminder", appErr, zap.Int64("id", id))
		return nil, appErr
	}
	return rem, nil
}

func (r *Repository) GetByIdempotencyKey(ctx context.Context, key uuid.UUID) (*reminder.Reminder, error) {
	const method = "GetByIdempotencyKey"
	start := time.Now()
	ctx, span := r.tracer.Start(ctx, "reminderrepo."+method)
	defer span.End()
	defer func() { r.met.RecordDuration(ctx, repoName, method, time.Since(start).Seconds()) }()

	rem, err := scanReminder(r.db.QueryRowContext(ctx, `SELECT id, user_id, medicine_id, schedule_id, scheduled_at, status, retry_count, postpone_count, idempotency_key, created_at FROM reminders WHERE idempotency_key=$1`, key))
	if errors.Is(err, sql.ErrNoRows) {
		repolog.Debug(ctx, r.log, "reminder not found by idempotency key", zap.String("key", key.String()))
		return nil, reminder.ErrNotFound
	}
	if err != nil {
		appErr := apperrors.DB("reminderrepo."+method, err)
		repolog.Err(ctx, r.log, span, r.met, repoName, method, "failed to get reminder by idempotency key", appErr, zap.String("key", key.String()))
		return nil, appErr
	}
	return rem, nil
}

func (r *Repository) ListByUser(ctx context.Context, userID int64, from, to time.Time) ([]reminder.Reminder, error) {
	const method = "ListByUser"
	start := time.Now()
	ctx, span := r.tracer.Start(ctx, "reminderrepo."+method)
	defer span.End()
	defer func() { r.met.RecordDuration(ctx, repoName, method, time.Since(start).Seconds()) }()

	result, err := queryList(ctx, r.db, `SELECT id, user_id, medicine_id, schedule_id, scheduled_at, status, retry_count, postpone_count, idempotency_key, created_at FROM reminders WHERE user_id=$1 AND scheduled_at BETWEEN $2 AND $3 ORDER BY scheduled_at DESC`, userID, from, to)
	if err != nil {
		appErr := apperrors.DB("reminderrepo."+method, err)
		repolog.Err(ctx, r.log, span, r.met, repoName, method, "failed to list reminders by user", appErr, zap.Int64("user_id", userID))
		return nil, appErr
	}
	return result, nil
}

func (r *Repository) ListPending(ctx context.Context, before time.Time, limit int) ([]reminder.Reminder, error) {
	const method = "ListPending"
	start := time.Now()
	ctx, span := r.tracer.Start(ctx, "reminderrepo."+method)
	defer span.End()
	defer func() { r.met.RecordDuration(ctx, repoName, method, time.Since(start).Seconds()) }()

	result, err := queryList(ctx, r.db, `SELECT id, user_id, medicine_id, schedule_id, scheduled_at, status, retry_count, postpone_count, idempotency_key, created_at FROM reminders WHERE status IN ('pending','sent') AND scheduled_at<=$1 ORDER BY scheduled_at ASC LIMIT $2`, before, limit)
	if err != nil {
		appErr := apperrors.DB("reminderrepo."+method, err)
		repolog.Err(ctx, r.log, span, r.met, repoName, method, "failed to list pending reminders", appErr)
		return nil, appErr
	}
	return result, nil
}

func (r *Repository) ListBySchedule(ctx context.Context, scheduleID int64, from, to time.Time) ([]reminder.Reminder, error) {
	const method = "ListBySchedule"
	start := time.Now()
	ctx, span := r.tracer.Start(ctx, "reminderrepo."+method)
	defer span.End()
	defer func() { r.met.RecordDuration(ctx, repoName, method, time.Since(start).Seconds()) }()

	result, err := queryList(ctx, r.db, `SELECT id, user_id, medicine_id, schedule_id, scheduled_at, status, retry_count, postpone_count, idempotency_key, created_at FROM reminders WHERE schedule_id=$1 AND scheduled_at BETWEEN $2 AND $3 ORDER BY scheduled_at ASC`, scheduleID, from, to)
	if err != nil {
		appErr := apperrors.DB("reminderrepo."+method, err)
		repolog.Err(ctx, r.log, span, r.met, repoName, method, "failed to list reminders by schedule", appErr, zap.Int64("schedule_id", scheduleID))
		return nil, appErr
	}
	return result, nil
}

func (r *Repository) Create(ctx context.Context, rem *reminder.Reminder) error {
	const method = "Create"
	start := time.Now()
	ctx, span := r.tracer.Start(ctx, "reminderrepo."+method)
	defer span.End()
	defer func() { r.met.RecordDuration(ctx, repoName, method, time.Since(start).Seconds()) }()

	err := r.db.QueryRowContext(ctx, `INSERT INTO reminders (user_id, medicine_id, schedule_id, scheduled_at, status, idempotency_key) VALUES ($1,$2,$3,$4,$5,$6) RETURNING id, created_at`,
		rem.UserID, rem.MedicineID, rem.ScheduleID, rem.ScheduledAt, rem.Status, rem.IdempotencyKey,
	).Scan(&rem.ID, &rem.CreatedAt)
	if err != nil {
		appErr := apperrors.DB("reminderrepo."+method, err)
		repolog.Err(ctx, r.log, span, r.met, repoName, method, "failed to create reminder", appErr,
			zap.Int64("user_id", rem.UserID),
			zap.Int64("schedule_id", rem.ScheduleID),
		)
		return appErr
	}
	return nil
}

func (r *Repository) UpdateStatus(ctx context.Context, id int64, scheduledAt time.Time, status reminder.Status) error {
	const method = "UpdateStatus"
	start := time.Now()
	ctx, span := r.tracer.Start(ctx, "reminderrepo."+method)
	defer span.End()
	defer func() { r.met.RecordDuration(ctx, repoName, method, time.Since(start).Seconds()) }()

	res, err := r.db.ExecContext(ctx, `UPDATE reminders SET status=$1 WHERE id=$2 AND scheduled_at=$3`, status, id, scheduledAt)
	if err != nil {
		appErr := apperrors.DB("reminderrepo."+method, err)
		repolog.Err(ctx, r.log, span, r.met, repoName, method, "failed to update reminder status", appErr,
			zap.Int64("id", id),
			zap.String("status", string(status)),
		)
		return appErr
	}
	if n, _ := res.RowsAffected(); n == 0 {
		repolog.Debug(ctx, r.log, "reminder not found on update status", zap.Int64("id", id))
		return reminder.ErrNotFound
	}
	return nil
}

func (r *Repository) IncrementRetry(ctx context.Context, id int64, scheduledAt time.Time) error {
	const method = "IncrementRetry"
	start := time.Now()
	ctx, span := r.tracer.Start(ctx, "reminderrepo."+method)
	defer span.End()
	defer func() { r.met.RecordDuration(ctx, repoName, method, time.Since(start).Seconds()) }()

	res, err := r.db.ExecContext(ctx, `UPDATE reminders SET retry_count=retry_count+1 WHERE id=$1 AND scheduled_at=$2`, id, scheduledAt)
	if err != nil {
		appErr := apperrors.DB("reminderrepo."+method, err)
		repolog.Err(ctx, r.log, span, r.met, repoName, method, "failed to increment retry count", appErr, zap.Int64("id", id))
		return appErr
	}
	if n, _ := res.RowsAffected(); n == 0 {
		repolog.Debug(ctx, r.log, "reminder not found on increment retry", zap.Int64("id", id))
		return reminder.ErrNotFound
	}
	return nil
}

func (r *Repository) IncrementPostpone(ctx context.Context, id int64, scheduledAt time.Time) error {
	const method = "IncrementPostpone"
	start := time.Now()
	ctx, span := r.tracer.Start(ctx, "reminderrepo."+method)
	defer span.End()
	defer func() { r.met.RecordDuration(ctx, repoName, method, time.Since(start).Seconds()) }()

	res, err := r.db.ExecContext(ctx, `UPDATE reminders SET postpone_count=postpone_count+1 WHERE id=$1 AND scheduled_at=$2`, id, scheduledAt)
	if err != nil {
		appErr := apperrors.DB("reminderrepo."+method, err)
		repolog.Err(ctx, r.log, span, r.met, repoName, method, "failed to increment postpone count", appErr, zap.Int64("id", id))
		return appErr
	}
	if n, _ := res.RowsAffected(); n == 0 {
		repolog.Debug(ctx, r.log, "reminder not found on increment postpone", zap.Int64("id", id))
		return reminder.ErrNotFound
	}
	return nil
}

type rowScanner interface{ Scan(dest ...any) error }

func scanReminder(s rowScanner) (*reminder.Reminder, error) {
	rem := &reminder.Reminder{}
	return rem, s.Scan(&rem.ID, &rem.UserID, &rem.MedicineID, &rem.ScheduleID, &rem.ScheduledAt, &rem.Status, &rem.RetryCount, &rem.PostponeCount, &rem.IdempotencyKey, &rem.CreatedAt)
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
