package medicine

import "errors"

var (
	ErrNotFound = errors.New("medicine: not found")

	ErrEmptyName = errors.New("medicine: name is required")
	
	ErrNameTooLong = errors.New("medicine: name is too long (max 256 chars)")
)
