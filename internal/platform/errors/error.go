package apperrors

import (
	"errors"
	"fmt"
)

type Severity string

const (
	SeverityInfo  Severity = "info"
	SeverityWarn  Severity = "warn"
	SeverityError Severity = "error"
	SeverityFatal Severity = "fatal"
)

type AppError struct {
	Code      string
	Module    string
	Severity  Severity
	Retryable bool
	Cause     error
}

func (e *AppError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("[%s] %s: %v", e.Module, e.Code, e.Cause)
	}
	return fmt.Sprintf("[%s] %s", e.Module, e.Code)
}

func (e *AppError) Unwrap() error { return e.Cause }

func Wrap(err error, code, module string) *AppError {
	return &AppError{Code: code, Module: module, Severity: SeverityError, Cause: err}
}

func Retryable(err error, code, module string) *AppError {
	return &AppError{Code: code, Module: module, Severity: SeverityWarn, Retryable: true, Cause: err}
}

func NewFatal(err error, code, module string) *AppError {
	return &AppError{Code: code, Module: module, Severity: SeverityFatal, Cause: err}
}

func IsRetryable(err error) bool {
	var ae *AppError
	return errors.As(err, &ae) && ae.Retryable
}

func CodeOf(err error) string {
	var ae *AppError
	if errors.As(err, &ae) {
		return ae.Code
	}
	return ""
}
