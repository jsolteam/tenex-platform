package user

import "errors"

var (
	ErrNotFound = errors.New("user: not found")

	ErrContactNotFound = errors.New("user: contact not found")

	ErrContactAlreadyExists = errors.New("user: contact already exists")

	ErrInvalidTimezone = errors.New("user: invalid timezone")

	ErrInvalidLanguage = errors.New("user: invalid language")
)
