package apperrors

import (
	"fmt"
	"strings"
)

// Attr — произвольная пара ключ/значение, добавляемая через With().
// Поля экспортированы для использования в логгере (zap.Any(a.Key, a.Val)).
type Attr struct {
	Key string
	Val any
}

// AppError — структурированная ошибка платформы.
type AppError struct {
	Code     ErrorCode
	Module   string
	Severity Severity
	Cause    error

	retryable bool
	attrs     []Attr
}

// Error реализует интерфейс error.
// Формат: [module] CODE key=val key=val: cause message
func (e *AppError) Error() string {
	var b strings.Builder
	fmt.Fprintf(&b, "[%s] %s", e.Module, e.Code)
	for _, a := range e.attrs {
		fmt.Fprintf(&b, " %s=%v", a.Key, a.Val)
	}
	if e.Cause != nil {
		b.WriteString(": ")
		b.WriteString(e.Cause.Error())
	}
	return b.String()
}

// Unwrap позволяет errors.Is / errors.As проходить сквозь AppError.
func (e *AppError) Unwrap() error { return e.Cause }

// Retryable помечает ошибку как повторяемую и переключает severity на Warn.
// Вызов идемпотентен — повторный вызов не меняет состояние.
func (e *AppError) Retryable() *AppError {
	e.retryable = true
	if e.Severity == SeverityError {
		e.Severity = SeverityWarn
	}
	return e
}

// Fatal переключает severity на Fatal.
// Используй для ошибок, после которых сервис не может продолжать работу.
func (e *AppError) Fatal() *AppError {
	e.Severity = SeverityFatal
	return e
}

// Info переключает severity на Info.
// Полезно для "ожидаемых" ошибок (not found, conflict), которые не должны
// создавать шум в мониторинге.
func (e *AppError) Info() *AppError {
	e.Severity = SeverityInfo
	return e
}

// With добавляет произвольный атрибут к ошибке.
// Атрибуты отображаются в Error() и могут быть использованы в логгере
// через appErr.Attrs() → zap.Any(a.Key, a.Val).
//
//	apperrors.DB("repo.List", err).With("user_id", uid).With("limit", 50)
func (e *AppError) With(key string, val any) *AppError {
	e.attrs = append(e.attrs, Attr{Key: key, Val: val})
	return e
}

// IsRetryable возвращает true если ошибка помечена как повторяемая.
func (e *AppError) IsRetryable() bool { return e.retryable }

// Attrs возвращает срез пар ключ/значение для использования в логгере:
func (e *AppError) Attrs() []Attr { return e.attrs }
