package apperrors

// DB создаёт ошибку запроса к базе данных (ErrDBQuery).
func DB(module string, cause error) *AppError {
	return &AppError{
		Code:     ErrDBQuery,
		Module:   module,
		Severity: SeverityError,
		Cause:    cause,
	}
}

// DBConnect создаёт ошибку подключения к базе данных (ErrDBConnect).
func DBConnect(module string, cause error) *AppError {
	return &AppError{
		Code:     ErrDBConnect,
		Module:   module,
		Severity: SeverityError,
		Cause:    cause,
	}
}

// Redis создаёт ошибку взаимодействия с Redis (ErrRedisUnavailable).
func Redis(module string, cause error) *AppError {
	return &AppError{
		Code:     ErrRedisUnavailable,
		Module:   module,
		Severity: SeverityError,
		Cause:    cause,
	}
}

// S3 создаёт ошибку взаимодействия с объектным хранилищем (ErrS3).
func S3(module string, cause error) *AppError {
	return &AppError{
		Code:     ErrS3,
		Module:   module,
		Severity: SeverityError,
		Cause:    cause,
	}
}

// External создаёт ошибку внешнего API — мессенджеры и т.п. (ErrMessengerAPI).
func External(module string, cause error) *AppError {
	return &AppError{
		Code:     ErrMessengerAPI,
		Module:   module,
		Severity: SeverityError,
		Cause:    cause,
	}
}

// Validation создаёт ошибку валидации входных данных (ErrValidation).
func Validation(module string, cause error) *AppError {
	return &AppError{
		Code:     ErrValidation,
		Module:   module,
		Severity: SeverityWarn,
		Cause:    cause,
	}
}

// InvalidInput создаёт ошибку некорректного ввода (ErrInvalidInput).
func InvalidInput(module string, cause error) *AppError {
	return &AppError{
		Code:     ErrInvalidInput,
		Module:   module,
		Severity: SeverityWarn,
		Cause:    cause,
	}
}

// Timeout создаёт ошибку истечения таймаута (ErrTimeout).
func Timeout(module string, cause error) *AppError {
	return &AppError{
		Code:      ErrTimeout,
		Module:    module,
		Severity:  SeverityWarn,
		Cause:     cause,
		retryable: true,
	}
}

// Canceled создаёт ошибку отмены контекста (ErrContextCanceled).
func Canceled(module string, cause error) *AppError {
	return &AppError{
		Code:     ErrContextCanceled,
		Module:   module,
		Severity: SeverityWarn,
		Cause:    cause,
	}
}

// Internal создаёт внутреннюю ошибку сервиса (ErrInternal).
func Internal(module string, cause error) *AppError {
	return &AppError{
		Code:     ErrInternal,
		Module:   module,
		Severity: SeverityError,
		Cause:    cause,
	}
}

// Panic создаёт ошибку из recovered panic (ErrPanic).
func Panic(module string, cause error) *AppError {
	return &AppError{
		Code:     ErrPanic,
		Module:   module,
		Severity: SeverityFatal,
		Cause:    cause,
	}
}

// Wrap создаёт AppError с произвольным кодом.
//
// Deprecated: используй именованные конструкторы — DB(), Validation(), Internal() и т.д.
func Wrap(cause error, code ErrorCode, module string) *AppError {
	return &AppError{Code: code, Module: module, Severity: SeverityError, Cause: cause}
}

// New создаёт AppError с произвольным кодом без оборачивания ошибки.
//
// Deprecated: используй именованные конструкторы — DB(), Validation(), Internal() и т.д.
func New(code ErrorCode, module string, cause error) *AppError {
	return &AppError{Code: code, Module: module, Severity: SeverityError, Cause: cause}
}

// NewFatal создаёт AppError с severity Fatal.
//
// Deprecated: используй Internal(module, cause).Fatal()
func NewFatal(cause error, code ErrorCode, module string) *AppError {
	return &AppError{Code: code, Module: module, Severity: SeverityFatal, Cause: cause}
}

// Retryable создаёт AppError помеченный как повторяемый с severity Warn.
//
// Deprecated: используй, например, DB(module, cause).Retryable()
func Retryable(cause error, code ErrorCode, module string) *AppError {
	return &AppError{
		Code:      code,
		Module:    module,
		Severity:  SeverityWarn,
		Cause:     cause,
		retryable: true,
	}
}
