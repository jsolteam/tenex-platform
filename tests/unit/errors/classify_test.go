package errors_unit

import (
	"errors"
	"fmt"
	"testing"

	apperrors "github.com/jsolteam/tenex-platform/internal/platform/errors"
)

func appErr(code apperrors.ErrorCode) error {
	return apperrors.Wrap(errors.New("cause"), code, "test")
}

func retryableErr(code apperrors.ErrorCode) error {
	return apperrors.Retryable(errors.New("cause"), code, "test")
}

func TestClassify_Validation_ErrValidation(t *testing.T) {
	if got := apperrors.Classify(appErr(apperrors.ErrValidation)); got != apperrors.CategoryValidation {
		t.Errorf("Classify(ErrValidation) = %q, want %q", got, apperrors.CategoryValidation)
	}
}

func TestClassify_Validation_ErrInvalidInput(t *testing.T) {
	if got := apperrors.Classify(appErr(apperrors.ErrInvalidInput)); got != apperrors.CategoryValidation {
		t.Errorf("Classify(ErrInvalidInput) = %q, want %q", got, apperrors.CategoryValidation)
	}
}

func TestClassify_Infrastructure_ErrDBConnect(t *testing.T) {
	if got := apperrors.Classify(appErr(apperrors.ErrDBConnect)); got != apperrors.CategoryInfrastructure {
		t.Errorf("Classify(ErrDBConnect) = %q, want %q", got, apperrors.CategoryInfrastructure)
	}
}

func TestClassify_Infrastructure_ErrDBQuery(t *testing.T) {
	if got := apperrors.Classify(appErr(apperrors.ErrDBQuery)); got != apperrors.CategoryInfrastructure {
		t.Errorf("Classify(ErrDBQuery) = %q, want %q", got, apperrors.CategoryInfrastructure)
	}
}

func TestClassify_Infrastructure_ErrRedisUnavailable(t *testing.T) {
	if got := apperrors.Classify(appErr(apperrors.ErrRedisUnavailable)); got != apperrors.CategoryInfrastructure {
		t.Errorf("Classify(ErrRedisUnavailable) = %q, want %q", got, apperrors.CategoryInfrastructure)
	}
}

func TestClassify_Infrastructure_ErrS3(t *testing.T) {
	if got := apperrors.Classify(appErr(apperrors.ErrS3)); got != apperrors.CategoryInfrastructure {
		t.Errorf("Classify(ErrS3) = %q, want %q", got, apperrors.CategoryInfrastructure)
	}
}

func TestClassify_Timeout_ErrTimeout(t *testing.T) {
	if got := apperrors.Classify(appErr(apperrors.ErrTimeout)); got != apperrors.CategoryTimeout {
		t.Errorf("Classify(ErrTimeout) = %q, want %q", got, apperrors.CategoryTimeout)
	}
}

func TestClassify_Timeout_ErrContextCanceled(t *testing.T) {
	if got := apperrors.Classify(appErr(apperrors.ErrContextCanceled)); got != apperrors.CategoryTimeout {
		t.Errorf("Classify(ErrContextCanceled) = %q, want %q", got, apperrors.CategoryTimeout)
	}
}

func TestClassify_External_ErrMessengerAPI(t *testing.T) {
	if got := apperrors.Classify(appErr(apperrors.ErrMessengerAPI)); got != apperrors.CategoryExternal {
		t.Errorf("Classify(ErrMessengerAPI) = %q, want %q", got, apperrors.CategoryExternal)
	}
}

func TestClassify_Internal_ErrInternal(t *testing.T) {
	if got := apperrors.Classify(appErr(apperrors.ErrInternal)); got != apperrors.CategoryInternal {
		t.Errorf("Classify(ErrInternal) = %q, want %q", got, apperrors.CategoryInternal)
	}
}

func TestClassify_Internal_ErrPanic(t *testing.T) {
	if got := apperrors.Classify(appErr(apperrors.ErrPanic)); got != apperrors.CategoryInternal {
		t.Errorf("Classify(ErrPanic) = %q, want %q", got, apperrors.CategoryInternal)
	}
}

func TestClassify_Internal_UnknownCode(t *testing.T) {
	unknown := apperrors.Wrap(errors.New("x"), apperrors.ErrorCode("UNKNOWN_FUTURE_CODE"), "test")
	if got := apperrors.Classify(unknown); got != apperrors.CategoryInternal {
		t.Errorf("Classify(unknown code) = %q, want %q", got, apperrors.CategoryInternal)
	}
}

func TestClassify_PlainError_ReturnsInternal(t *testing.T) {
	if got := apperrors.Classify(errors.New("plain")); got != apperrors.CategoryInternal {
		t.Errorf("Classify(plain error) = %q, want %q", got, apperrors.CategoryInternal)
	}
}

func TestClassify_Nil_ReturnsInternal(t *testing.T) {
	if got := apperrors.Classify(nil); got != apperrors.CategoryInternal {
		t.Errorf("Classify(nil) = %q, want %q", got, apperrors.CategoryInternal)
	}
}

func TestClassify_WrappedAppError_StillClassifies(t *testing.T) {
	inner := apperrors.Wrap(errors.New("cause"), apperrors.ErrTimeout, "db")
	outer := fmt.Errorf("outer wrapper: %w", inner)
	if got := apperrors.Classify(outer); got != apperrors.CategoryTimeout {
		t.Errorf("Classify(wrapped AppError) = %q, want %q", got, apperrors.CategoryTimeout)
	}
}

func TestClassify_AllInfrastructureCodes(t *testing.T) {
	infraCodes := []apperrors.ErrorCode{
		apperrors.ErrDBConnect,
		apperrors.ErrDBQuery,
		apperrors.ErrRedisUnavailable,
		apperrors.ErrS3,
	}
	for _, code := range infraCodes {
		got := apperrors.Classify(appErr(code))
		if got != apperrors.CategoryInfrastructure {
			t.Errorf("Classify(%q) = %q, want %q", code, got, apperrors.CategoryInfrastructure)
		}
	}
}

func TestClassify_AllTimeoutCodes(t *testing.T) {
	for _, code := range []apperrors.ErrorCode{apperrors.ErrTimeout, apperrors.ErrContextCanceled} {
		got := apperrors.Classify(appErr(code))
		if got != apperrors.CategoryTimeout {
			t.Errorf("Classify(%q) = %q, want %q", code, got, apperrors.CategoryTimeout)
		}
	}
}

func TestIsTemporary_Alias_RetryableError(t *testing.T) {
	err := retryableErr(apperrors.ErrTimeout)
	if !apperrors.IsTemporary(err) {
		t.Error("IsTemporary(retryable) should return true")
	}
}

func TestIsTemporary_Alias_NonRetryableError(t *testing.T) {
	err := apperrors.Wrap(errors.New("x"), apperrors.ErrDBConnect, "db")
	if apperrors.IsTemporary(err) {
		t.Error("IsTemporary(non-retryable Wrap) should return false")
	}
}

func TestIsTemporary_Alias_PlainError(t *testing.T) {
	if apperrors.IsTemporary(errors.New("plain")) {
		t.Error("IsTemporary(plain) should return false")
	}
}

func TestIsTemporary_Alias_Nil(t *testing.T) {
	if apperrors.IsTemporary(nil) {
		t.Error("IsTemporary(nil) should return false")
	}
}

func TestIsTemporary_MatchesIsRetryable(t *testing.T) {
	errs := []error{
		retryableErr(apperrors.ErrTimeout),
		appErr(apperrors.ErrDBConnect),
		errors.New("plain"),
		nil,
	}
	for _, err := range errs {
		got := apperrors.IsTemporary(err)
		want := apperrors.IsRetryable(err)
		if got != want {
			t.Errorf("IsTemporary(%v) = %v, IsRetryable = %v — they must match", err, got, want)
		}
	}
}

func TestRetryable_SetsTemporaryField(t *testing.T) {
	err := apperrors.Retryable(errors.New("x"), apperrors.ErrTimeout, "svc")
	if !err.Temporary {
		t.Error("Retryable() should set Temporary = true")
	}
}

func TestWrap_TemporaryFieldFalse(t *testing.T) {
	err := apperrors.Wrap(errors.New("x"), apperrors.ErrDBConnect, "db")
	if err.Temporary {
		t.Error("Wrap() should leave Temporary = false")
	}
}

func TestNew_CreatesAppError(t *testing.T) {
	cause := errors.New("root cause")
	err := apperrors.New(apperrors.ErrInternal, "worker", cause)
	if err == nil {
		t.Fatal("New returned nil")
	}
	if err.Code != apperrors.ErrInternal {
		t.Errorf("Code = %q, want %q", err.Code, apperrors.ErrInternal)
	}
	if err.Module != "worker" {
		t.Errorf("Module = %q, want %q", err.Module, "worker")
	}
	if !errors.Is(err, cause) {
		t.Error("errors.Is should find original cause")
	}
}

func TestErrorCode_Constants_AreDistinct(t *testing.T) {
	codes := []apperrors.ErrorCode{
		apperrors.ErrValidation,
		apperrors.ErrInvalidInput,
		apperrors.ErrDBConnect,
		apperrors.ErrDBQuery,
		apperrors.ErrRedisUnavailable,
		apperrors.ErrTimeout,
		apperrors.ErrContextCanceled,
		apperrors.ErrMessengerAPI,
		apperrors.ErrS3,
		apperrors.ErrInternal,
		apperrors.ErrPanic,
	}
	seen := make(map[apperrors.ErrorCode]bool)
	for _, c := range codes {
		if seen[c] {
			t.Errorf("duplicate ErrorCode value: %q", c)
		}
		seen[c] = true
	}
}

func TestCodeOf_ReturnsErrorCode(t *testing.T) {
	err := apperrors.Wrap(errors.New("x"), apperrors.ErrMessengerAPI, "bot")
	got := apperrors.CodeOf(err)
	if got != apperrors.ErrMessengerAPI {
		t.Errorf("CodeOf = %q, want %q", got, apperrors.ErrMessengerAPI)
	}
}
