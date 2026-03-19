package schedulerepo

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/lib/pq"

	"github.com/jsolteam/tenex-platform/internal/domain/schedule"
	apperrors "github.com/jsolteam/tenex-platform/internal/platform/errors"
)

type Repository struct {
	db *sql.DB
}

func New(db *sql.DB) *Repository {
	return &Repository{db: db}
}

// GetByID возвращает расписание по ID.
func (r *Repository) GetByID(ctx context.Context, id int64) (*schedule.Schedule, error) {
	const q = `
		SELECT id, medicine_id, schedule_type, interval_days, days_of_week, times, start_date, end_date, created_at
		FROM schedules
		WHERE id = $1`

	s, err := r.scanRow(r.db.QueryRowContext(ctx, q, id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, schedule.ErrNotFound
	}
	if err != nil {
		return nil, apperrors.DB("schedulerepo.GetByID", err)
	}
	return s, nil
}

// ListByMedicine возвращает все расписания для лекарства.
func (r *Repository) ListByMedicine(ctx context.Context, medicineID int64) ([]schedule.Schedule, error) {
	const q = `
		SELECT id, medicine_id, schedule_type, interval_days, days_of_week, times, start_date, end_date, created_at
		FROM schedules
		WHERE medicine_id = $1
		ORDER BY created_at ASC`

	result, err := r.queryList(ctx, q, medicineID)
	if err != nil {
		return nil, apperrors.DB("schedulerepo.ListByMedicine", err)
	}
	return result, nil
}

// ListActive возвращает расписания активные на указанную дату.
func (r *Repository) ListActive(ctx context.Context, date time.Time) ([]schedule.Schedule, error) {
	const q = `
		SELECT id, medicine_id, schedule_type, interval_days, days_of_week, times, start_date, end_date, created_at
		FROM schedules
		WHERE start_date <= $1 AND (end_date IS NULL OR end_date >= $1)
		ORDER BY id ASC`

	result, err := r.queryList(ctx, q, date.Truncate(24*time.Hour))
	if err != nil {
		return nil, apperrors.DB("schedulerepo.ListActive", err)
	}
	return result, nil
}

// Create создаёт новое расписание.
func (r *Repository) Create(ctx context.Context, s *schedule.Schedule) error {
	const q = `
		INSERT INTO schedules (medicine_id, schedule_type, interval_days, days_of_week, times, start_date, end_date)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, created_at`

	err := r.db.QueryRowContext(ctx, q,
		s.MedicineID,
		s.Type,
		s.IntervalDays,
		s.DaysOfWeek,
		pq.Array(timesToStrings(s.Times)),
		s.StartDate,
		s.EndDate,
	).Scan(&s.ID, &s.CreatedAt)
	if err != nil {
		return apperrors.DB("schedulerepo.Create", err)
	}
	return nil
}

// Update обновляет расписание.
func (r *Repository) Update(ctx context.Context, s *schedule.Schedule) error {
	const q = `
		UPDATE schedules
		SET schedule_type = $1, interval_days = $2, days_of_week = $3,
		    times = $4, start_date = $5, end_date = $6
		WHERE id = $7`

	res, err := r.db.ExecContext(ctx, q,
		s.Type, s.IntervalDays, s.DaysOfWeek,
		pq.Array(timesToStrings(s.Times)),
		s.StartDate, s.EndDate, s.ID,
	)
	if err != nil {
		return apperrors.DB("schedulerepo.Update", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return schedule.ErrNotFound
	}
	return nil
}

// Delete удаляет расписание.
func (r *Repository) Delete(ctx context.Context, id int64) error {
	const q = `DELETE FROM schedules WHERE id = $1`

	res, err := r.db.ExecContext(ctx, q, id)
	if err != nil {
		return apperrors.DB("schedulerepo.Delete", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
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

type scanner interface {
	Scan(dest ...any) error
}

func (r *Repository) scanRow(s scanner) (*schedule.Schedule, error) {
	var row schedule.Schedule
	var intervalDays sql.NullInt16
	var daysOfWeek sql.NullInt16
	var endDate sql.NullTime
	var rawTimes []string

	err := s.Scan(
		&row.ID, &row.MedicineID, &row.Type,
		&intervalDays, &daysOfWeek,
		pq.Array(&rawTimes),
		&row.StartDate, &endDate,
		&row.CreatedAt,
	)
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
		t, err := time.Parse("15:04:05", s)
		if err == nil {
			out = append(out, t)
		}
	}
	return out
}
