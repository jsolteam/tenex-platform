package userrepo

import (
	"context"
	"database/sql"
	"errors"

	"github.com/lib/pq"
	"go.uber.org/zap"

	"github.com/jsolteam/tenex-platform/internal/domain/user"
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
		log:    log.With(zap.String("repo", "user")),
		tracer: tracer,
	}
}

func (r *Repository) GetByID(ctx context.Context, id int64) (*user.User, error) {
	ctx, span := r.tracer.Start(ctx, "userrepo.GetByID")
	defer span.End()

	const q = `SELECT id, timezone, language, created_at, updated_at FROM users WHERE id = $1`

	u := &user.User{}
	err := r.db.QueryRowContext(ctx, q, id).Scan(
		&u.ID, &u.Timezone, &u.Language, &u.CreatedAt, &u.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		repolog.Debug(ctx, r.log, "user not found", zap.Int64("id", id))
		return nil, user.ErrNotFound
	}
	if err != nil {
		appErr := apperrors.DB("userrepo.GetByID", err)
		repolog.Err(ctx, r.log, span, "failed to get user", appErr, zap.Int64("id", id))
		return nil, appErr
	}
	return u, nil
}

func (r *Repository) GetByMessenger(ctx context.Context, messengerType user.MessengerType, messengerUserID string) (*user.User, error) {
	ctx, span := r.tracer.Start(ctx, "userrepo.GetByMessenger")
	defer span.End()

	const q = `
		SELECT u.id, u.timezone, u.language, u.created_at, u.updated_at
		FROM users u
		JOIN user_contacts uc ON uc.user_id = u.id
		WHERE uc.messenger_type = $1 AND uc.messenger_user_id = $2`

	u := &user.User{}
	err := r.db.QueryRowContext(ctx, q, messengerType, messengerUserID).Scan(
		&u.ID, &u.Timezone, &u.Language, &u.CreatedAt, &u.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		repolog.Debug(ctx, r.log, "user not found by messenger",
			zap.String("messenger_type", string(messengerType)),
			zap.String("messenger_user_id", messengerUserID),
		)
		return nil, user.ErrNotFound
	}
	if err != nil {
		appErr := apperrors.DB("userrepo.GetByMessenger", err)
		repolog.Err(ctx, r.log, span, "failed to get user by messenger", appErr,
			zap.String("messenger_type", string(messengerType)),
		)
		return nil, appErr
	}
	return u, nil
}

func (r *Repository) Create(ctx context.Context, u *user.User, contact *user.UserContact) (retErr error) {
	ctx, span := r.tracer.Start(ctx, "userrepo.Create")
	defer span.End()

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		appErr := apperrors.DBConnect("userrepo.Create.begin", err)
		repolog.Err(ctx, r.log, span, "failed to begin transaction", appErr)
		return appErr
	}
	defer func() {
		if retErr != nil {
			_ = tx.Rollback()
		}
	}()

	const qUser = `
		INSERT INTO users (timezone, language) VALUES ($1, $2)
		RETURNING id, created_at, updated_at`

	if err = tx.QueryRowContext(ctx, qUser, u.Timezone, u.Language).
		Scan(&u.ID, &u.CreatedAt, &u.UpdatedAt); err != nil {
		appErr := apperrors.DB("userrepo.Create.insertUser", err)
		repolog.Err(ctx, r.log, span, "failed to insert user", appErr)
		retErr = appErr
		return
	}

	contact.UserID = u.ID
	contact.IsPrimary = true

	const qContact = `
		INSERT INTO user_contacts (user_id, messenger_type, messenger_user_id, username, is_primary)
		VALUES ($1, $2, $3, $4, $5) RETURNING id, created_at`

	if err = tx.QueryRowContext(ctx, qContact,
		contact.UserID, contact.MessengerType, contact.MessengerUserID,
		contact.Username, contact.IsPrimary,
	).Scan(&contact.ID, &contact.CreatedAt); err != nil {
		if isUniqueViolation(err) {
			appErr := apperrors.Validation("userrepo.Create.insertContact", user.ErrContactAlreadyExists)
			repolog.Err(ctx, r.log, span, "contact already exists on create", appErr,
				zap.String("messenger_type", string(contact.MessengerType)),
			)
			retErr = appErr
			return
		}
		appErr := apperrors.DB("userrepo.Create.insertContact", err)
		repolog.Err(ctx, r.log, span, "failed to insert contact", appErr)
		retErr = appErr
		return
	}

	if err = tx.Commit(); err != nil {
		appErr := apperrors.DB("userrepo.Create.commit", err)
		repolog.Err(ctx, r.log, span, "failed to commit create user", appErr)
		retErr = appErr
	}
	return
}

func (r *Repository) Update(ctx context.Context, u *user.User) error {
	ctx, span := r.tracer.Start(ctx, "userrepo.Update")
	defer span.End()

	const q = `
		UPDATE users SET timezone = $1, language = $2, updated_at = now()
		WHERE id = $3 RETURNING updated_at`

	err := r.db.QueryRowContext(ctx, q, u.Timezone, u.Language, u.ID).Scan(&u.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		repolog.Debug(ctx, r.log, "user not found on update", zap.Int64("id", u.ID))
		return user.ErrNotFound
	}
	if err != nil {
		appErr := apperrors.DB("userrepo.Update", err)
		repolog.Err(ctx, r.log, span, "failed to update user", appErr, zap.Int64("id", u.ID))
		return appErr
	}
	return nil
}

func (r *Repository) GetContacts(ctx context.Context, userID int64) ([]user.UserContact, error) {
	ctx, span := r.tracer.Start(ctx, "userrepo.GetContacts")
	defer span.End()

	const q = `
		SELECT id, user_id, messenger_type, messenger_user_id, username, is_primary, created_at
		FROM user_contacts WHERE user_id = $1
		ORDER BY is_primary DESC, created_at ASC`

	rows, err := r.db.QueryContext(ctx, q, userID)
	if err != nil {
		appErr := apperrors.DB("userrepo.GetContacts", err)
		repolog.Err(ctx, r.log, span, "failed to query contacts", appErr, zap.Int64("user_id", userID))
		return nil, appErr
	}
	defer rows.Close()

	var contacts []user.UserContact
	for rows.Next() {
		var c user.UserContact
		if err = rows.Scan(
			&c.ID, &c.UserID, &c.MessengerType, &c.MessengerUserID,
			&c.Username, &c.IsPrimary, &c.CreatedAt,
		); err != nil {
			appErr := apperrors.DB("userrepo.GetContacts.scan", err)
			repolog.Err(ctx, r.log, span, "failed to scan contact", appErr, zap.Int64("user_id", userID))
			return nil, appErr
		}
		contacts = append(contacts, c)
	}
	if err = rows.Err(); err != nil {
		appErr := apperrors.DB("userrepo.GetContacts.rows", err)
		repolog.Err(ctx, r.log, span, "contacts rows error", appErr, zap.Int64("user_id", userID))
		return nil, appErr
	}
	return contacts, nil
}

func (r *Repository) AddContact(ctx context.Context, contact *user.UserContact) error {
	ctx, span := r.tracer.Start(ctx, "userrepo.AddContact")
	defer span.End()

	const q = `
		INSERT INTO user_contacts (user_id, messenger_type, messenger_user_id, username, is_primary)
		VALUES ($1, $2, $3, $4, $5) RETURNING id, created_at`

	err := r.db.QueryRowContext(ctx, q,
		contact.UserID, contact.MessengerType, contact.MessengerUserID,
		contact.Username, contact.IsPrimary,
	).Scan(&contact.ID, &contact.CreatedAt)
	if err != nil {
		if isUniqueViolation(err) {
			appErr := apperrors.Validation("userrepo.AddContact", user.ErrContactAlreadyExists)
			repolog.Err(ctx, r.log, span, "contact already exists", appErr,
				zap.Int64("user_id", contact.UserID),
				zap.String("messenger_type", string(contact.MessengerType)),
			)
			return appErr
		}
		appErr := apperrors.DB("userrepo.AddContact", err)
		repolog.Err(ctx, r.log, span, "failed to add contact", appErr, zap.Int64("user_id", contact.UserID))
		return appErr
	}
	return nil
}

func (r *Repository) SetPrimaryContact(ctx context.Context, userID, contactID int64) (retErr error) {
	ctx, span := r.tracer.Start(ctx, "userrepo.SetPrimaryContact")
	defer span.End()

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		appErr := apperrors.DBConnect("userrepo.SetPrimaryContact.begin", err)
		repolog.Err(ctx, r.log, span, "failed to begin transaction", appErr)
		return appErr
	}
	defer func() {
		if retErr != nil {
			_ = tx.Rollback()
		}
	}()

	if _, err = tx.ExecContext(ctx,
		`UPDATE user_contacts SET is_primary = false WHERE user_id = $1`, userID,
	); err != nil {
		appErr := apperrors.DB("userrepo.SetPrimaryContact.reset", err)
		repolog.Err(ctx, r.log, span, "failed to reset primary contacts", appErr, zap.Int64("user_id", userID))
		retErr = appErr
		return
	}

	var id int64
	if err = tx.QueryRowContext(ctx,
		`UPDATE user_contacts SET is_primary = true WHERE id = $1 AND user_id = $2 RETURNING id`,
		contactID, userID,
	).Scan(&id); errors.Is(err, sql.ErrNoRows) {
		repolog.Debug(ctx, r.log, "contact not found on set primary",
			zap.Int64("user_id", userID),
			zap.Int64("contact_id", contactID),
		)
		retErr = user.ErrContactNotFound
		return
	} else if err != nil {
		appErr := apperrors.DB("userrepo.SetPrimaryContact.set", err)
		repolog.Err(ctx, r.log, span, "failed to set primary contact", appErr,
			zap.Int64("user_id", userID),
			zap.Int64("contact_id", contactID),
		)
		retErr = appErr
		return
	}

	if err = tx.Commit(); err != nil {
		appErr := apperrors.DB("userrepo.SetPrimaryContact.commit", err)
		repolog.Err(ctx, r.log, span, "failed to commit set primary", appErr)
		retErr = appErr
	}
	return
}

func isUniqueViolation(err error) bool {
	var pqErr *pq.Error
	return errors.As(err, &pqErr) && pqErr.Code == "23505"
}
