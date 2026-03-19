package apperrors

type ErrorCode string

const (
	// ── Database ─────────────────────────────────────────────────────────────
	ErrDBConnect ErrorCode = "DB_CONNECT"
	ErrDBQuery   ErrorCode = "DB_QUERY"

	// ── Cache / Redis ─────────────────────────────────────────────────────────
	ErrRedisUnavailable ErrorCode = "REDIS_UNAVAILABLE"

	// ── Object storage ────────────────────────────────────────────────────────
	ErrS3 ErrorCode = "S3_ERROR"

	// ── External integrations ─────────────────────────────────────────────────
	ErrMessengerAPI ErrorCode = "MESSENGER_API"

	// ── Validation / input ────────────────────────────────────────────────────
	ErrValidation   ErrorCode = "VALIDATION"
	ErrInvalidInput ErrorCode = "INVALID_INPUT"

	// ── Timing ───────────────────────────────────────────────────────────────
	ErrTimeout         ErrorCode = "TIMEOUT"
	ErrContextCanceled ErrorCode = "CTX_CANCELED"

	// ── Catch-all ────────────────────────────────────────────────────────────
	ErrInternal ErrorCode = "INTERNAL"
	ErrPanic    ErrorCode = "PANIC"
)

type Severity string

const (
	SeverityInfo  Severity = "info"
	SeverityWarn  Severity = "warn"
	SeverityError Severity = "error"
	SeverityFatal Severity = "fatal"
)

type Category string

const (
	CategoryValidation     Category = "validation"
	CategoryInfrastructure Category = "infrastructure"
	CategoryTimeout        Category = "timeout"
	CategoryExternal       Category = "external"
	CategoryInternal       Category = "internal"
)
