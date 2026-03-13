package errors_unit

import (
	"errors"
	"strings"
	"testing"

	apperrors "github.com/jsolteam/tenex-platform/internal/platform/errors"
)

func TestWrap_CreatesAppError(t *testing.T) {
	cause := errors.New("connection refused")
	err := apperrors.Wrap(cause, "DB_CONNECT", "database")

	if err == nil {
		t.Fatal("Wrap returned nil")
	}
	if err.Code != "DB_CONNECT" {
		t.Errorf("Code = %q, want %q", err.Code, "DB_CONNECT")
	}
	if err.Module != "database" {
		t.Errorf("Module = %q, want %q", err.Module, "database")
	}
	if err.Severity != apperrors.SeverityError {
		t.Errorf("Severity = %q, want %q", err.Severity, apperrors.SeverityError)
	}
}

func TestWrap_ErrorMessage(t *testing.T) {
	cause := errors.New("disk full")
	err := apperrors.Wrap(cause, "IO_ERR", "storage")
	msg := err.Error()

	for _, want := range []string{"IO_ERR", "storage", "disk full"} {
		if !strings.Contains(msg, want) {
			t.Errorf("Error() missing %q: %s", want, msg)
		}
	}
}

func TestWrap_Unwrap(t *testing.T) {
	cause := errors.New("original")
	err := apperrors.Wrap(cause, "C", "m")
	if !errors.Is(err, cause) {
		t.Error("errors.Is should find original cause via Unwrap()")
	}
}

func TestWrap_NilCause(t *testing.T) {
	err := apperrors.Wrap(nil, "CODE", "mod")
	if err == nil {
		t.Fatal("Wrap(nil) returned nil")
	}
	_ = err.Error()
}

func TestRetryable_IsRetryable(t *testing.T) {
	err := apperrors.Retryable(errors.New("timeout"), "TIMEOUT", "http")
	if !apperrors.IsRetryable(err) {
		t.Error("Retryable() error should be retryable")
	}
	if err.Severity != apperrors.SeverityWarn {
		t.Errorf("Severity = %q, want %q", err.Severity, apperrors.SeverityWarn)
	}
}

func TestWrap_NotRetryable(t *testing.T) {
	err := apperrors.Wrap(errors.New("x"), "C", "m")
	if apperrors.IsRetryable(err) {
		t.Error("Wrap() error should NOT be retryable")
	}
}

func TestIsRetryable_PlainError(t *testing.T) {
	if apperrors.IsRetryable(errors.New("plain")) {
		t.Error("plain error should not be retryable")
	}
}

func TestIsRetryable_Nil(t *testing.T) {
	if apperrors.IsRetryable(nil) {
		t.Error("nil should not be retryable")
	}
}

func TestNewFatal_ReturnsAppError(t *testing.T) {
	cause := errors.New("disk full")
	err := apperrors.NewFatal(cause, "FATAL_IO", "storage")

	if err == nil {
		t.Fatal("NewFatal returned nil")
	}
	if err.Severity != apperrors.SeverityFatal {
		t.Errorf("Severity = %q, want %q", err.Severity, apperrors.SeverityFatal)
	}
	if !errors.Is(err, cause) {
		t.Error("errors.Is should find cause")
	}
}

func TestNewFatal_DoesNotCallOsExit(t *testing.T) {
	_ = apperrors.NewFatal(errors.New("x"), "C", "m")
	t.Log("NewFatal correctly does NOT call os.Exit")
}

func TestCodeOf_AppError(t *testing.T) {
	err := apperrors.Wrap(errors.New("x"), "MY_CODE", "m")
	if got := apperrors.CodeOf(err); got != "MY_CODE" {
		t.Errorf("CodeOf = %q, want %q", got, "MY_CODE")
	}
}

func TestCodeOf_PlainError(t *testing.T) {
	if got := apperrors.CodeOf(errors.New("x")); got != "" {
		t.Errorf("CodeOf(plain) = %q, want empty", got)
	}
}

func TestCodeOf_Nil(t *testing.T) {
	if got := apperrors.CodeOf(nil); got != "" {
		t.Errorf("CodeOf(nil) = %q, want empty", got)
	}
}

func TestAppError_ErrorWithoutCause(t *testing.T) {
	err := &apperrors.AppError{Code: "CODE", Module: "mod"}
	msg := err.Error()
	if !strings.Contains(msg, "CODE") || !strings.Contains(msg, "mod") {
		t.Errorf("unexpected message: %q", msg)
	}
}
