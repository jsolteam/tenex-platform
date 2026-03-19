package userrepo

import (
	"context"
	"database/sql"
	"errors"

	"github.com/lib/pq"

	"github.com/jsolteam/tenex-platform/internal/domain/user"
	apperrors "github.com/jsolteam/tenex-platform/internal/platform/errors"
)

type Repository struct {
	db *sql.DB
}

func New(db *sql.DB) *Repository {
	return &Repository{db: db}
}

// GetByID возвращает пользователя по ID.
func (r *Repository) GetByID(ctx context.Context, id int64) (*user.User, error) {
	const q = `
		SELECT id, timezone, language, created_at, updated_at
		FROM users
		WHERE id = $1`

	u := &user.User{}
	err := r.db.QueryRowContext(ctx, q, id).Scan(
		&u.ID, &u.Timezone, &u.Language, &u.CreatedAt, &u.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, user.ErrNotFound
	}
	if err != nil {
		return nil, apperrors.DB("userrepo.GetByID", err)
	}
	return u, nil
}

// GetByMessenger возвращает пользователя по идентификатору в мессенджере.
func (r *Repository) GetByMessenger(ctx context.Context, messengerType user.MessengerType, messengerUserID string) (*user.User, error) {
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
		return nil, user.ErrNotFound
	}
	if err != nil {
		return nil, apperrors.DB("userrepo.GetByMessenger", err)
	}
	return u, nil
}

// Create создаёт нового пользователя вместе с первичным контактом в одной транзакции.
func (r *Repository) Create(ctx context.Context, u *user.User, contact *user.UserContact) (retErr error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return apperrors.DBConnect("userrepo.Create.begin", err)
	}
	defer func() {
		if retErr != nil {
			_ = tx.Rollback()
		}
	}()

	const qUser = `
		INSERT INTO users (timezone, language)
		VALUES ($1, $2)
		RETURNING id, created_at, updated_at`

	if err = tx.QueryRowContext(ctx, qUser, u.Timezone, u.Language).
		Scan(&u.ID, &u.CreatedAt, &u.UpdatedAt); err != nil {
		retErr = apperrors.DB("userrepo.Create.insertUser", err)
		return
	}

	contact.UserID = u.ID
	contact.IsPrimary = true

	const qContact = `
		INSERT INTO user_contacts (user_id, messenger_type, messenger_user_id, username, is_primary)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, created_at`

	if err = tx.QueryRowContext(ctx, qContact,
		contact.UserID, contact.MessengerType, contact.MessengerUserID,
		contact.Username, contact.IsPrimary,
	).Scan(&contact.ID, &contact.CreatedAt); err != nil {
		if isUniqueViolation(err) {
			retErr = apperrors.Validation("userrepo.Create.insertContact", user.ErrContactAlreadyExists)
			return
		}
		retErr = apperrors.DB("userrepo.Create.insertContact", err)
		return
	}

	retErr = tx.Commit()
	if retErr != nil {
		retErr = apperrors.DB("userrepo.Create.commit", retErr)
	}
	return
}

// Update обновляет timezone и language пользователя.
func (r *Repository) Update(ctx context.Context, u *user.User) error {
	const q = `
		UPDATE users
		SET timezone = $1, language = $2, updated_at = now()
		WHERE id = $3
		RETURNING updated_at`

	err := r.db.QueryRowContext(ctx, q, u.Timezone, u.Language, u.ID).
		Scan(&u.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return user.ErrNotFound
	}
	if err != nil {
		return apperrors.DB("userrepo.Update", err)
	}
	return nil
}

// GetContacts возвращает все контакты пользователя.
func (r *Repository) GetContacts(ctx context.Context, userID int64) ([]user.UserContact, error) {
	const q = `
		SELECT id, user_id, messenger_type, messenger_user_id, username, is_primary, created_at
		FROM user_contacts
		WHERE user_id = $1
		ORDER BY is_primary DESC, created_at ASC`

	rows, err := r.db.QueryContext(ctx, q, userID)
	if err != nil {
		return nil, apperrors.DB("userrepo.GetContacts", err)
	}
	defer rows.Close()

	var contacts []user.UserContact
	for rows.Next() {
		var c user.UserContact
		if err = rows.Scan(
			&c.ID, &c.UserID, &c.MessengerType, &c.MessengerUserID,
			&c.Username, &c.IsPrimary, &c.CreatedAt,
		); err != nil {
			return nil, apperrors.DB("userrepo.GetContacts.scan", err)
		}
		contacts = append(contacts, c)
	}
	if err = rows.Err(); err != nil {
		return nil, apperrors.DB("userrepo.GetContacts.rows", err)
	}
	return contacts, nil
}

// AddContact добавляет новый контакт пользователю.
func (r *Repository) AddContact(ctx context.Context, contact *user.UserContact) error {
	const q = `
		INSERT INTO user_contacts (user_id, messenger_type, messenger_user_id, username, is_primary)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, created_at`

	err := r.db.QueryRowContext(ctx, q,
		contact.UserID, contact.MessengerType, contact.MessengerUserID,
		contact.Username, contact.IsPrimary,
	).Scan(&contact.ID, &contact.CreatedAt)
	if err != nil {
		if isUniqueViolation(err) {
			// Оборачиваем доменную ошибку в AppError с кодом ErrValidation,
			// чтобы Classify() и CodeOf() работали корректно на всех слоях.
			return apperrors.Validation("userrepo.AddContact", user.ErrContactAlreadyExists)
		}
		return apperrors.DB("userrepo.AddContact", err)
	}
	return nil
}

// SetPrimaryContact делает указанный контакт основным.
func (r *Repository) SetPrimaryContact(ctx context.Context, userID, contactID int64) (retErr error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return apperrors.DBConnect("userrepo.SetPrimaryContact.begin", err)
	}
	defer func() {
		if retErr != nil {
			_ = tx.Rollback()
		}
	}()

	if _, err = tx.ExecContext(ctx,
		`UPDATE user_contacts SET is_primary = false WHERE user_id = $1`, userID,
	); err != nil {
		retErr = apperrors.DB("userrepo.SetPrimaryContact.reset", err)
		return
	}

	var id int64
	if err = tx.QueryRowContext(ctx,
		`UPDATE user_contacts SET is_primary = true WHERE id = $1 AND user_id = $2 RETURNING id`,
		contactID, userID,
	).Scan(&id); errors.Is(err, sql.ErrNoRows) {
		retErr = user.ErrContactNotFound
		return
	} else if err != nil {
		retErr = apperrors.DB("userrepo.SetPrimaryContact.set", err)
		return
	}

	if err = tx.Commit(); err != nil {
		retErr = apperrors.DB("userrepo.SetPrimaryContact.commit", err)
	}
	return
}

// ── helpers ───────────────────────────────────────────────────────────────

// isUniqueViolation проверяет что ошибка — нарушение уникального индекса (PostgreSQL code 23505).
func isUniqueViolation(err error) bool {
	var pqErr *pq.Error
	return errors.As(err, &pqErr) && pqErr.Code == "23505"
}
