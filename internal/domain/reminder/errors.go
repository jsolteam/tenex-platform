package reminder

import "errors"

var (
	ErrNotFound = errors.New("reminder: not found")

	ErrAlreadyExists = errors.New("reminder: already exists")

	ErrInvalidStatus = errors.New("reminder: invalid status transition")

	ErrFinalStatus = errors.New("reminder: already in final status")
	
	ErrMaxRetriesReached = errors.New("reminder: max retries reached")
)
