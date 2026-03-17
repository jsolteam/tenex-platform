package intake

import "errors"

var (
	ErrNotFound = errors.New("intake: not found")

	ErrAlreadyExists = errors.New("intake: already exists for this reminder")

	ErrReasonRequired = errors.New("intake: reason is required when skipping")
	
	ErrInvalidStatus = errors.New("intake: invalid status")
)
