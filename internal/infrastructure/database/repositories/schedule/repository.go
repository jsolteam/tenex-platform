package schedulerepo

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/lib/pq"
	"go.uber.org/zap"

	"github.com/jsolteam/tenex-platform/internal/domain/schedule"
	"github.com/jsolteam/tenex-platform/internal/infrastructure/repolog"
	apperrors "github.com/jsolteam/tenex-platform/internal/platform/errors"
	"github.com/jsolteam/tenex-platform/internal/platform/logger/core"
	"github.com/jsolteam/tenex-platform/internal/platform/observability/metrics"
	"github.com/jsolteam/tenex-platform/internal/platform/observability/tracing"
)

const repoName = "schedule"

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

func (r *Repository) GetByID(ctx context.Context, id int64) (*schedule.Schedule, error) {
	const method = "GetByID"
	start := time.Now()
	ctx, span := r.tracer.Start(ctx, "schedulerepo."+method)
	defer span.End()
	defer func() { r.met.RecordDuration(ctx, repoName, method, time.Since(start).Seconds()) }()

	const q = `SELECT id, medicine_id, schedule_type, interval_days, days_of_week, times, start_date, end_date, created_at FROM schedules WHERE id = $1`

	s, err := r.scanRow(r.db.QueryRowContext(ctx, q, id))
	if errors.Is(err, sql.ErrNoRows) {
		repolog.Debug(ctx, r.log, "schedule not found", zap.Int64("id", id))
		return nil, schedule.ErrNotFound
	}
	if err != nil {
		appErr := apperrors.DB("schedulerepo."+method, err)
		repolog.Err(ctx, r.log, span, r.met, repoName, method, "failed to get schedule", appErr, zap.Int64("id", id))
		return nil, appErr
	}
	return s, nil
}

func (r *Repository) ListByMedicine(ctx context.Context, medicineID int64) ([]schedule.Schedule, error) {
	const method = "ListByMedicine"
	start := time.Now()
	ctx, span := r.tracer.Start(ctx, "schedulerepo."+method)
	defer span.End()
	defer func() { r.met.RecordDuration(ctx, repoName, method, time.Since(start).Seconds()) }()

	const q = `SELECT id, medicine_id, schedule_type, interval_days, days_of_week, times, start_date, end_date, created_at FROM schedules WHERE medicine_id = $1 ORDER BY created_at ASC`

	result, err := r.queryList(ctx, q, medicineID)
	if err != nil {
		appErr := apperrors.DB("schedulerepo."+method, err)
		repolog.Err(ctx, r.log, span, r.met, repoName, method, "failed to list schedules", appErr, zap.Int64("medicine_id", medicineID))
		return nil, appErr
	}
	return result, nil
}

func (r *Repository) ListActive(ctx context.Context, date time.Time) ([]schedule.Schedule, error) {
	const method = "ListActive"
	start := time.Now()
	ctx, span := r.tracer.Start(ctx, "schedulerepo."+method)
	defer span.End()
	defer func() { r.met.RecordDuration(ctx, repoName, method, time.Since(start).Seconds()) }()

	const q = `SELECT id, medicine_id, schedule_type, interval_days, days_of_week, times, start_date, end_date, created_at FROM schedules WHERE start_date <= $1 AND (end_date IS NULL OR end_date >= $1) ORDER BY id ASC`

	result, err := r.queryList(ctx, q, date.Truncate(24*time.Hour))
	if err != nil {
		appErr := apperrors.DB("schedulerepo."+method, err)
		repolog.Err(ctx, r.log, span, r.met, repoName, method, "failed to list active schedules", appErr, zap.Time("date", date))
		return nil, appErr
	}
	return result, nil
}

func (r *Repository) Create(ctx context.Context, s *schedule.Schedule) error {
	const method = "Create"
	start := time.Now()
	ctx, span := r.tracer.Start(ctx, "schedulerepo."+method)
	defer span.End()
	defer func() { r.met.RecordDuration(ctx, repoName, method, time.Since(start).Seconds()) }()

	const q = `INSERT INTO schedules (medicine_id, schedule_type, interval_days, days_of_week, times, start_date, end_date) VALUES ($1,$2,$3,$4,$5,$6,$7) RETURNING id, created_at`

	err := r.db.QueryRowContext(ctx, q,
		s.MedicineID, s.Type, s.IntervalDays, s.DaysOfWeek,
		pq.Array(timesToStrings(s.Times)), s.StartDate, s.EndDate,
	).Scan(&s.ID, &s.CreatedAt)
	if err != nil {
		appErr := apperrors.DB("schedulerepo."+method, err)
		repolog.Err(ctx, r.log, span, r.met, repoName, method, "failed to create schedule", appErr,
			zap.Int64("medicine_id", s.MedicineID),
			zap.String("type", string(s.Type)),
		)
		return appErr
	}
	return nil
}

func (r *Repository) Update(ctx context.Context, s *schedule.Schedule) error {
	const method = "Update"
	start := time.Now()
	ctx, span := r.tracer.Start(ctx, "schedulerepo."+method)
	defer span.End()
	defer func() { r.met.RecordDuration(ctx, repoName, method, time.Since(start).Seconds()) }()

	res, err := r.db.ExecContext(ctx, `UPDATE schedules SET schedule_type=$1, interval_days=$2, days_of_week=$3, times=$4, start_date=$5, end_date=$6 WHERE id=$7`,
		s.Type, s.IntervalDays, s.DaysOfWeek, pq.Array(timesToStrings(s.Times)), s.StartDate, s.EndDate, s.ID,
	)
	if err != nil {
		appErr := apperrors.DB("schedulerepo."+method, err)
		repolog.Err(ctx, r.log, span, r.met, repoName, method, "failed to update schedule", appErr, zap.Int64("id", s.ID))
		return appErr
	}
	if n, _ := res.RowsAffected(); n == 0 {
		repolog.Debug(ctx, r.log, "schedule not found on update", zap.Int64("id", s.ID))
		return schedule.ErrNotFound
	}
	return nil
}

func (r *Repository) Delete(ctx context.Context, id int64) error {
	const method = "Delete"
	start := time.Now()
	ctx, span := r.tracer.Start(ctx, "schedulerepo."+method)
	defer span.End()
	defer func() { r.met.RecordDuration(ctx, repoName, method, time.Since(start).Seconds()) }()

	res, err := r.db.ExecContext(ctx, `DELETE FROM schedules WHERE id = $1`, id)
	if err != nil {
		appErr := apperrors.DB("schedulerepo."+method, err)
		repolog.Err(ctx, r.log, span, r.met, repoName, method, "failed to delete schedule", appErr, zap.Int64("id", id))
		return appErr
	}
	if n, _ := res.RowsAffected(); n == 0 {
		repolog.Debug(ctx, r.log, "schedule not found on delete", zap.Int64("id", id))
		return schedule.ErrNotFound
	}
	return nil
}

// ── helpers ───────────────────────────────────────────────────────────────

func (r *Repository) queryList(ctx context.Context, q string, args ...any) ([]schedule.Schedule, error) {
	rows, err := r.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []schedule.Schedule
	for rows.Next() {
		s, err := r.scanRow(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, *s)
	}
	return result, rows.Err()
}

type rowScanner interface{ Scan(dest ...any) error }

func (r *Repository) scanRow(s rowScanner) (*schedule.Schedule, error) {
	var row schedule.Schedule
	var intervalDays sql.NullInt16
	var daysOfWeek sql.NullInt16
	var endDate sql.NullTime
	var rawTimes []string
	err := s.Scan(&row.ID, &row.MedicineID, &row.Type, &intervalDays, &daysOfWeek, pq.Array(&rawTimes), &row.StartDate, &endDate, &row.CreatedAt)
	if err != nil {
		return nil, err
	}
	if intervalDays.Valid {
		v := intervalDays.Int16
		row.IntervalDays = &v
	}
	if daysOfWeek.Valid {
		v := schedule.DayOfWeek(daysOfWeek.Int16)
		row.DaysOfWeek = &v
	}
	if endDate.Valid {
		row.EndDate = &endDate.Time
	}
	row.Times = stringsToTimes(rawTimes)
	return &row, nil
}

func timesToStrings(times []time.Time) []string {
	out := make([]string, len(times))
	for i, t := range times {
		out[i] = t.Format("15:04:05")
	}
	return out
}

func stringsToTimes(ss []string) []time.Time {
	out := make([]time.Time, 0, len(ss))
	for _, s := range ss {
		if t, err := time.Parse("15:04:05", s); err == nil {
			out = append(out, t)
		}
	}
	return out
}
