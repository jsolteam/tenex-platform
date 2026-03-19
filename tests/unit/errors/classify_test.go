package errors_unit

import (
	"errors"
	"fmt"
	"testing"

	apperrors "github.com/jsolteam/tenex-platform/internal/platform/errors"
)

func TestClassify_DB_ReturnsInfrastructure(t *testing.T) {
	cases := []error{
		apperrors.DB("repo", errors.New("x")),
		apperrors.DBConnect("db", errors.New("x")),
	}
	for _, err := range cases {
		if got := apperrors.Classify(err); got != apperrors.CategoryInfrastructure {
			t.Errorf("Classify(%v) = %q, want infrastructure", err, got)
		}
	}
}

func TestClassify_Redis_ReturnsInfrastructure(t *testing.T) {
	err := apperrors.Redis("cache", errors.New("x"))
	if got := apperrors.Classify(err); got != apperrors.CategoryInfrastructure {
		t.Errorf("Classify(Redis) = %q, want infrastructure", got)
	}
}

func TestClassify_S3_ReturnsInfrastructure(t *testing.T) {
	err := apperrors.S3("media", errors.New("x"))
	if got := apperrors.Classify(err); got != apperrors.CategoryInfrastructure {
		t.Errorf("Classify(S3) = %q, want infrastructure", got)
	}
}

func TestClassify_External_ReturnsExternal(t *testing.T) {
	err := apperrors.External("telegram", errors.New("429"))
	if got := apperrors.Classify(err); got != apperrors.CategoryExternal {
		t.Errorf("Classify(External) = %q, want external", got)
	}
}

func TestClassify_Validation_ReturnsValidation(t *testing.T) {
	cases := []error{
		apperrors.Validation("repo", errors.New("x")),
		apperrors.InvalidInput("handler", errors.New("x")),
	}
	for _, err := range cases {
		if got := apperrors.Classify(err); got != apperrors.CategoryValidation {
			t.Errorf("Classify(%v) = %q, want validation", err, got)
		}
	}
}

func TestClassify_Timeout_ReturnsTimeout(t *testing.T) {
	cases := []error{
		apperrors.Timeout("http", errors.New("x")),
		apperrors.Canceled("ctx", errors.New("x")),
	}
	for _, err := range cases {
		if got := apperrors.Classify(err); got != apperrors.CategoryTimeout {
			t.Errorf("Classify(%v) = %q, want timeout", err, got)
		}
	}
}

func TestClassify_Internal_ReturnsInternal(t *testing.T) {
	cases := []error{
		apperrors.Internal("worker", errors.New("x")),
		apperrors.Panic("goroutine", errors.New("nil deref")),
	}
	for _, err := range cases {
		if got := apperrors.Classify(err); got != apperrors.CategoryInternal {
			t.Errorf("Classify(%v) = %q, want internal", err, got)
		}
	}
}

func TestClassify_PlainError_ReturnsInternal(t *testing.T) {
	if got := apperrors.Classify(errors.New("plain")); got != apperrors.CategoryInternal {
		t.Errorf("Classify(plain) = %q, want internal", got)
	}
}

func TestClassify_Nil_ReturnsInternal(t *testing.T) {
	if got := apperrors.Classify(nil); got != apperrors.CategoryInternal {
		t.Errorf("Classify(nil) = %q, want internal", got)
	}
}

func TestClassify_WrappedAppError_StillClassifies(t *testing.T) {
	inner := apperrors.Timeout("db", errors.New("deadline"))
	outer := fmt.Errorf("outer: %w", inner)
	if got := apperrors.Classify(outer); got != apperrors.CategoryTimeout {
		t.Errorf("Classify(wrapped) = %q, want timeout", got)
	}
}

func TestClassify_AllInfrastructureCodes(t *testing.T) {
	infraErrs := []error{
		apperrors.DB("r", errors.New("x")),
		apperrors.DBConnect("r", errors.New("x")),
		apperrors.Redis("r", errors.New("x")),
		apperrors.S3("r", errors.New("x")),
	}
	for _, err := range infraErrs {
		if got := apperrors.Classify(err); got != apperrors.CategoryInfrastructure {
			t.Errorf("Classify(%v) = %q, want infrastructure", apperrors.CodeOf(err), got)
		}
	}
}

// ── Retryable через fluent-цепочку ────────────────────────────────────────

func TestIsRetryable_FluentChain(t *testing.T) {
	retryable := apperrors.DB("repo", errors.New("deadlock")).Retryable()
	nonRetryable := apperrors.DB("repo", errors.New("constraint"))

	if !apperrors.IsRetryable(retryable) {
		t.Error("DB().Retryable() should be retryable")
	}
	if apperrors.IsRetryable(nonRetryable) {
		t.Error("DB() without .Retryable() should not be retryable")
	}
}

func TestIsRetryable_TimeoutIsAlwaysRetryable(t *testing.T) {
	err := apperrors.Timeout("http", errors.New("deadline"))
	if !apperrors.IsRetryable(err) {
		t.Error("Timeout errors are retryable by default")
	}
}

func TestIsRetryable_ExternalNotRetryableByDefault(t *testing.T) {
	err := apperrors.External("telegram", errors.New("400 Bad Request"))
	if apperrors.IsRetryable(err) {
		t.Error("External errors are NOT retryable by default — must explicitly call .Retryable()")
	}
}

func TestIsRetryable_ExternalCanBeMadeRetryable(t *testing.T) {
	err := apperrors.External("telegram", errors.New("429 Too Many Requests")).Retryable()
	if !apperrors.IsRetryable(err) {
		t.Error("External().Retryable() should be retryable")
	}
}

// ── Deprecated compat functions ───────────────────────────────────────────

func TestClassify_DeprecatedWrap_StillWorks(t *testing.T) {
	cases := []struct {
		err  error
		want apperrors.Category
	}{
		{apperrors.Wrap(errors.New("x"), apperrors.ErrValidation, "m"), apperrors.CategoryValidation},
		{apperrors.Wrap(errors.New("x"), apperrors.ErrDBQuery, "m"), apperrors.CategoryInfrastructure},
		{apperrors.Wrap(errors.New("x"), apperrors.ErrTimeout, "m"), apperrors.CategoryTimeout},
		{apperrors.Wrap(errors.New("x"), apperrors.ErrMessengerAPI, "m"), apperrors.CategoryExternal},
		{apperrors.Wrap(errors.New("x"), apperrors.ErrInternal, "m"), apperrors.CategoryInternal},
	}
	for _, tc := range cases {
		if got := apperrors.Classify(tc.err); got != tc.want {
			t.Errorf("Classify(Wrap(%v)) = %q, want %q", apperrors.CodeOf(tc.err), got, tc.want)
		}
	}
}
