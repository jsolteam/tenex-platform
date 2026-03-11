package apperrors

import "errors"

type Category string

const (
	CategoryValidation     Category = "validation"
	CategoryInfrastructure Category = "infrastructure"
	CategoryTimeout        Category = "timeout"
	CategoryExternal       Category = "external"
	CategoryInternal       Category = "internal"
)

func Classify(err error) Category {
	var ae *AppError
	if !errors.As(err, &ae) {
		return CategoryInternal
	}
	switch ae.Code {
	case ErrValidation, ErrInvalidInput:
		return CategoryValidation
	case ErrDBConnect, ErrDBQuery, ErrRedisUnavailable, ErrS3:
		return CategoryInfrastructure
	case ErrTimeout, ErrContextCanceled:
		return CategoryTimeout
	case ErrMessengerAPI:
		return CategoryExternal
	default:
		return CategoryInternal
	}
}

// IsTemporary is an alias for IsRetryable kept for backwards compatibility.
// Prefer IsRetryable in new code — "retryable" more precisely describes what
// the flag means: the caller may safely retry the operation. "Temporary" is
// ambiguous (it could mean the error resolves on its own without a retry).
//
// Deprecated: use IsRetryable.
func IsTemporary(err error) bool {
	return IsRetryable(err)
}
