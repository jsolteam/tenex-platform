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

func IsTemporary(err error) bool {
	return IsRetryable(err)
}
