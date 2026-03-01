package apperrors

type ErrorCode string

// Predefined error codes grouped by subsystem.
const (
	// Validation / input
	ErrValidation   ErrorCode = "VALIDATION"
	ErrInvalidInput ErrorCode = "INVALID_INPUT"

	// Database
	ErrDBConnect ErrorCode = "DB_CONNECT"
	ErrDBQuery   ErrorCode = "DB_QUERY"

	// Redis
	ErrRedisUnavailable ErrorCode = "REDIS_UNAVAILABLE"

	// Timing
	ErrTimeout         ErrorCode = "TIMEOUT"
	ErrContextCanceled ErrorCode = "CTX_CANCELED"

	// External integrations
	ErrMessengerAPI ErrorCode = "MESSENGER_API"

	// Object storage
	ErrS3 ErrorCode = "S3_ERROR"

	// Catch-all
	ErrInternal ErrorCode = "INTERNAL"
	ErrPanic    ErrorCode = "PANIC"
)
