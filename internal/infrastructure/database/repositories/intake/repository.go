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
	"github.com/jsolteam/tenex-platform/internal/platform/observability/tracing"
)

type Repository struct {
	db     *sql.DB
	log    *core.Logger
	tracer tracing.Tracer
}

func New(db *sql.DB, log *core.Logger, tracer tracing.Tracer) *Repository {
	return &Repository{
		db:     db,
		log:    log.With(zap.String("repo", "intake")),
		tracer: tracer,
	}
}

func (r *Repository) GetByID(ctx context.Context, id int64) (*intake.Intake, error) {
	ctx, span := r.tracer.Start(ctx, "intakerepo.GetByID")
	defer span.End()

	const q = `SELECT id, reminder_id, status, proof_media_id, COALESCE(reason,''), created_at FROM intakes WHERE id = $1`

	i := &intake.Intake{}
	err := r.db.QueryRowContext(ctx, q, id).Scan(&i.ID, &i.ReminderID, &i.Status, &i.ProofMediaID, &i.Reason, &i.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		repolog.Debug(ctx, r.log, "intake not found", zap.Int64("id", id))
		return nil, intake.ErrNotFound
	}
	if err != nil {
		appErr := apperrors.DB("intakerepo.GetByID", err)
		repolog.Err(ctx, r.log, span, "failed to get intake", appErr, zap.Int64("id", id))
		return nil, appErr
	}
	return i, nil
}

func (r *Repository) GetByReminderID(ctx context.Context, reminderID int64) (*intake.Intake, error) {
	ctx, span := r.tracer.Start(ctx, "intakerepo.GetByReminderID")
	defer span.End()

	const q = `SELECT id, reminder_id, status, proof_media_id, COALESCE(reason,''), created_at FROM intakes WHERE reminder_id = $1`

	i := &intake.Intake{}
	err := r.db.QueryRowContext(ctx, q, reminderID).Scan(&i.ID, &i.ReminderID, &i.Status, &i.ProofMediaID, &i.Reason, &i.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		repolog.Debug(ctx, r.log, "intake not found by reminder", zap.Int64("reminder_id", reminderID))
		return nil, intake.ErrNotFound
	}
	if err != nil {
		appErr := apperrors.DB("intakerepo.GetByReminderID", err)
		repolog.Err(ctx, r.log, span, "failed to get intake by reminder", appErr, zap.Int64("reminder_id", reminderID))
		return nil, appErr
	}
	return i, nil
}

func (r *Repository) ListByReminders(ctx context.Context, reminderIDs []int64) ([]intake.Intake, error) {
	ctx, span := r.tracer.Start(ctx, "intakerepo.ListByReminders")
	defer span.End()

	const q = `SELECT id, reminder_id, status, proof_media_id, COALESCE(reason,''), created_at FROM intakes WHERE reminder_id = ANY($1) ORDER BY created_at DESC`

	rows, err := r.db.QueryContext(ctx, q, pq.Array(reminderIDs))
	if err != nil {
		appErr := apperrors.DB("intakerepo.ListByReminders", err)
		repolog.Err(ctx, r.log, span, "failed to list intakes by reminders", appErr, zap.Int("count", len(reminderIDs)))
		return nil, appErr
	}
	defer rows.Close()

	result, err := scanIntakes(rows)
	if err != nil {
		appErr := apperrors.DB("intakerepo.ListByReminders.scan", err)
		repolog.Err(ctx, r.log, span, "failed to scan intakes", appErr)
		return nil, appErr
	}
	return result, nil
}

func (r *Repository) ListConfirmedWithProof(ctx context.Context, from, to time.Time, limit int) ([]intake.Intake, error) {
	ctx, span := r.tracer.Start(ctx, "intakerepo.ListConfirmedWithProof")
	defer span.End()

	const q = `
		SELECT id, reminder_id, status, proof_media_id, COALESCE(reason,''), created_at
		FROM intakes WHERE status='confirmed' AND proof_media_id IS NOT NULL AND created_at BETWEEN $1 AND $2
		ORDER BY created_at DESC LIMIT $3`

	rows, err := r.db.QueryContext(ctx, q, from, to, limit)
	if err != nil {
		appErr := apperrors.DB("intakerepo.ListConfirmedWithProof", err)
		repolog.Err(ctx, r.log, span, "failed to list confirmed intakes with proof", appErr)
		return nil, appErr
	}
	defer rows.Close()

	result, err := scanIntakes(rows)
	if err != nil {
		appErr := apperrors.DB("intakerepo.ListConfirmedWithProof.scan", err)
		repolog.Err(ctx, r.log, span, "failed to scan confirmed intakes", appErr)
		return nil, appErr
	}
	return result, nil
}

func (r *Repository) Create(ctx context.Context, i *intake.Intake) error {
	ctx, span := r.tracer.Start(ctx, "intakerepo.Create")
	defer span.End()

	const q = `INSERT INTO intakes (reminder_id, status, proof_media_id, reason) VALUES ($1,$2,$3,NULLIF($4,'')) RETURNING id, created_at`

	err := r.db.QueryRowContext(ctx, q, i.ReminderID, i.Status, i.ProofMediaID, i.Reason).Scan(&i.ID, &i.CreatedAt)
	if err != nil {
		appErr := apperrors.DB("intakerepo.Create", err)
		repolog.Err(ctx, r.log, span, "failed to create intake", appErr,
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
