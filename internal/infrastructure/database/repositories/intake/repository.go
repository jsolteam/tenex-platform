package intakerepo

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/lib/pq"

	"github.com/jsolteam/tenex-platform/internal/domain/intake"
	apperrors "github.com/jsolteam/tenex-platform/internal/platform/errors"
)

type Repository struct {
	db *sql.DB
}

func New(db *sql.DB) *Repository {
	return &Repository{db: db}
}

// GetByID возвращает запись о приёме по ID.
func (r *Repository) GetByID(ctx context.Context, id int64) (*intake.Intake, error) {
	const q = `
		SELECT id, reminder_id, status, proof_media_id, COALESCE(reason, ''), created_at
		FROM intakes
		WHERE id = $1`

	i := &intake.Intake{}
	err := r.db.QueryRowContext(ctx, q, id).Scan(
		&i.ID, &i.ReminderID, &i.Status, &i.ProofMediaID, &i.Reason, &i.CreatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, intake.ErrNotFound
	}
	if err != nil {
		return nil, apperrors.DB("intakerepo.GetByID", err)
	}
	return i, nil
}

// GetByReminderID возвращает запись о приёме для напоминания.
func (r *Repository) GetByReminderID(ctx context.Context, reminderID int64) (*intake.Intake, error) {
	const q = `
		SELECT id, reminder_id, status, proof_media_id, COALESCE(reason, ''), created_at
		FROM intakes
		WHERE reminder_id = $1`

	i := &intake.Intake{}
	err := r.db.QueryRowContext(ctx, q, reminderID).Scan(
		&i.ID, &i.ReminderID, &i.Status, &i.ProofMediaID, &i.Reason, &i.CreatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, intake.ErrNotFound
	}
	if err != nil {
		return nil, apperrors.DB("intakerepo.GetByReminderID", err)
	}
	return i, nil
}

// ListByReminders возвращает записи о приёмах для нескольких напоминаний.
func (r *Repository) ListByReminders(ctx context.Context, reminderIDs []int64) ([]intake.Intake, error) {
	const q = `
		SELECT id, reminder_id, status, proof_media_id, COALESCE(reason, ''), created_at
		FROM intakes
		WHERE reminder_id = ANY($1)
		ORDER BY created_at DESC`

	rows, err := r.db.QueryContext(ctx, q, pq.Array(reminderIDs))
	if err != nil {
		return nil, apperrors.DB("intakerepo.ListByReminders", err)
	}
	defer rows.Close()

	result, err := scanIntakes(rows)
	if err != nil {
		return nil, apperrors.DB("intakerepo.ListByReminders.scan", err)
	}
	return result, nil
}

// ListConfirmedWithProof возвращает подтверждённые приёмы с медиа за период.
func (r *Repository) ListConfirmedWithProof(ctx context.Context, from, to time.Time, limit int) ([]intake.Intake, error) {
	const q = `
		SELECT id, reminder_id, status, proof_media_id, COALESCE(reason, ''), created_at
		FROM intakes
		WHERE status = 'confirmed' AND proof_media_id IS NOT NULL
		  AND created_at BETWEEN $1 AND $2
		ORDER BY created_at DESC
		LIMIT $3`

	rows, err := r.db.QueryContext(ctx, q, from, to, limit)
	if err != nil {
		return nil, apperrors.DB("intakerepo.ListConfirmedWithProof", err)
	}
	defer rows.Close()

	result, err := scanIntakes(rows)
	if err != nil {
		return nil, apperrors.DB("intakerepo.ListConfirmedWithProof.scan", err)
	}
	return result, nil
}

// Create создаёт запись о приёме/пропуске.
func (r *Repository) Create(ctx context.Context, i *intake.Intake) error {
	const q = `
		INSERT INTO intakes (reminder_id, status, proof_media_id, reason)
		VALUES ($1, $2, $3, NULLIF($4, ''))
		RETURNING id, created_at`

	err := r.db.QueryRowContext(ctx, q,
		i.ReminderID, i.Status, i.ProofMediaID, i.Reason,
	).Scan(&i.ID, &i.CreatedAt)
	if err != nil {
		return apperrors.DB("intakerepo.Create", err)
	}
	return nil
}

// ── helpers ───────────────────────────────────────────────────────────────

func scanIntakes(rows *sql.Rows) ([]intake.Intake, error) {
	var result []intake.Intake
	for rows.Next() {
		var i intake.Intake
		if err := rows.Scan(
			&i.ID, &i.ReminderID, &i.Status, &i.ProofMediaID, &i.Reason, &i.CreatedAt,
		); err != nil {
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
	result := "{"
	for i, v := range a {
		if i > 0 {
			result += ","
		}
		result += fmt.Sprintf("%d", v)
	}
	result += "}"
	return result, nil
}
