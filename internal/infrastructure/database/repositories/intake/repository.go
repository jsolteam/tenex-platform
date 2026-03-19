package intakerepo

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/lib/pq"
	"go.uber.org/zap"

	"github.com/jsolteam/tenex-platform/internal/domain/intake"
	"github.com/jsolteam/tenex-platform/internal/infrastructure/repolog"
	apperrors "github.com/jsolteam/tenex-platform/internal/platform/errors"
	"github.com/jsolteam/tenex-platform/internal/platform/logger/core"
	"github.com/jsolteam/tenex-platform/internal/platform/observability/metrics"
	"github.com/jsolteam/tenex-platform/internal/platform/observability/tracing"
)

const repoName = "intake"

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

func (r *Repository) GetByID(ctx context.Context, id int64) (*intake.Intake, error) {
	const method = "GetByID"
	start := time.Now()
	ctx, span := r.tracer.Start(ctx, "intakerepo."+method)
	defer span.End()
	defer func() { r.met.RecordDuration(ctx, repoName, method, time.Since(start).Seconds()) }()

	i := &intake.Intake{}
	err := r.db.QueryRowContext(ctx, `SELECT id, reminder_id, status, proof_media_id, COALESCE(reason,''), created_at FROM intakes WHERE id=$1`, id).
		Scan(&i.ID, &i.ReminderID, &i.Status, &i.ProofMediaID, &i.Reason, &i.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		repolog.Debug(ctx, r.log, "intake not found", zap.Int64("id", id))
		return nil, intake.ErrNotFound
	}
	if err != nil {
		appErr := apperrors.DB("intakerepo."+method, err)
		repolog.Err(ctx, r.log, span, r.met, repoName, method, "failed to get intake", appErr, zap.Int64("id", id))
		return nil, appErr
	}
	return i, nil
}

func (r *Repository) GetByReminderID(ctx context.Context, reminderID int64) (*intake.Intake, error) {
	const method = "GetByReminderID"
	start := time.Now()
	ctx, span := r.tracer.Start(ctx, "intakerepo."+method)
	defer span.End()
	defer func() { r.met.RecordDuration(ctx, repoName, method, time.Since(start).Seconds()) }()

	i := &intake.Intake{}
	err := r.db.QueryRowContext(ctx, `SELECT id, reminder_id, status, proof_media_id, COALESCE(reason,''), created_at FROM intakes WHERE reminder_id=$1`, reminderID).
		Scan(&i.ID, &i.ReminderID, &i.Status, &i.ProofMediaID, &i.Reason, &i.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		repolog.Debug(ctx, r.log, "intake not found by reminder", zap.Int64("reminder_id", reminderID))
		return nil, intake.ErrNotFound
	}
	if err != nil {
		appErr := apperrors.DB("intakerepo."+method, err)
		repolog.Err(ctx, r.log, span, r.met, repoName, method, "failed to get intake by reminder", appErr, zap.Int64("reminder_id", reminderID))
		return nil, appErr
	}
	return i, nil
}

func (r *Repository) ListByReminders(ctx context.Context, reminderIDs []int64) ([]intake.Intake, error) {
	const method = "ListByReminders"
	start := time.Now()
	ctx, span := r.tracer.Start(ctx, "intakerepo."+method)
	defer span.End()
	defer func() { r.met.RecordDuration(ctx, repoName, method, time.Since(start).Seconds()) }()

	rows, err := r.db.QueryContext(ctx, `SELECT id, reminder_id, status, proof_media_id, COALESCE(reason,''), created_at FROM intakes WHERE reminder_id=ANY($1) ORDER BY created_at DESC`, pq.Array(reminderIDs))
	if err != nil {
		appErr := apperrors.DB("intakerepo."+method, err)
		repolog.Err(ctx, r.log, span, r.met, repoName, method, "failed to list intakes by reminders", appErr, zap.Int("count", len(reminderIDs)))
		return nil, appErr
	}
	defer rows.Close()
	result, err := scanIntakes(rows)
	if err != nil {
		appErr := apperrors.DB("intakerepo."+method+".scan", err)
		repolog.Err(ctx, r.log, span, r.met, repoName, method, "failed to scan intakes", appErr)
		return nil, appErr
	}
	return result, nil
}

func (r *Repository) ListConfirmedWithProof(ctx context.Context, from, to time.Time, limit int) ([]intake.Intake, error) {
	const method = "ListConfirmedWithProof"
	start := time.Now()
	ctx, span := r.tracer.Start(ctx, "intakerepo."+method)
	defer span.End()
	defer func() { r.met.RecordDuration(ctx, repoName, method, time.Since(start).Seconds()) }()

	rows, err := r.db.QueryContext(ctx, `SELECT id, reminder_id, status, proof_media_id, COALESCE(reason,''), created_at FROM intakes WHERE status='confirmed' AND proof_media_id IS NOT NULL AND created_at BETWEEN $1 AND $2 ORDER BY created_at DESC LIMIT $3`, from, to, limit)
	if err != nil {
		appErr := apperrors.DB("intakerepo."+method, err)
		repolog.Err(ctx, r.log, span, r.met, repoName, method, "failed to list confirmed intakes with proof", appErr)
		return nil, appErr
	}
	defer rows.Close()
	result, err := scanIntakes(rows)
	if err != nil {
		appErr := apperrors.DB("intakerepo."+method+".scan", err)
		repolog.Err(ctx, r.log, span, r.met, repoName, method, "failed to scan confirmed intakes", appErr)
		return nil, appErr
	}
	return result, nil
}

func (r *Repository) Create(ctx context.Context, i *intake.Intake) error {
	const method = "Create"
	start := time.Now()
	ctx, span := r.tracer.Start(ctx, "intakerepo."+method)
	defer span.End()
	defer func() { r.met.RecordDuration(ctx, repoName, method, time.Since(start).Seconds()) }()

	err := r.db.QueryRowContext(ctx, `INSERT INTO intakes (reminder_id, status, proof_media_id, reason) VALUES ($1,$2,$3,NULLIF($4,'')) RETURNING id, created_at`,
		i.ReminderID, i.Status, i.ProofMediaID, i.Reason,
	).Scan(&i.ID, &i.CreatedAt)
	if err != nil {
		appErr := apperrors.DB("intakerepo."+method, err)
		repolog.Err(ctx, r.log, span, r.met, repoName, method, "failed to create intake", appErr,
			zap.Int64("reminder_id", i.ReminderID),
			zap.String("status", string(i.Status)),
		)
		return appErr
	}
	return nil
}

func scanIntakes(rows *sql.Rows) ([]intake.Intake, error) {
	var result []intake.Intake
	for rows.Next() {
		var i intake.Intake
		if err := rows.Scan(&i.ID, &i.ReminderID, &i.Status, &i.ProofMediaID, &i.Reason, &i.CreatedAt); err != nil {
			return nil, err
		}
		result = append(result, i)
	}
	return result, rows.Err()
}

type pqInt64Array []int64

func (a pqInt64Array) Value() (interface{}, error) {
	if len(a) == 0 {
		return "{}", nil
	}
	s := "{"
	for i, v := range a {
		if i > 0 {
			s += ","
		}
		s += fmt.Sprintf("%d", v)
	}
	return s + "}", nil
}
