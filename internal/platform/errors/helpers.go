package apperrors

import "errors"

// CodeOf возвращает ErrorCode из AppError или пустую строку,
// если err не является или не содержит *AppError.
func CodeOf(err error) ErrorCode {
	var ae *AppError
	if errors.As(err, &ae) {
		return ae.Code
	}
	return ""
}

// IsRetryable возвращает true если err является или содержит *AppError
// с флагом retryable == true.
func IsRetryable(err error) bool {
	var ae *AppError
	return errors.As(err, &ae) && ae.retryable
}

// IsTemporary — псевдоним IsRetryable для обратной совместимости.
//
// Deprecated: используй IsRetryable.
func IsTemporary(err error) bool {
	return IsRetryable(err)
}

// Classify возвращает семантическую категорию ошибки.
// Если err не *AppError — возвращает CategoryInternal.
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
