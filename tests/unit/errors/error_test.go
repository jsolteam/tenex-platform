package errors_unit

import (
	"errors"
	"strings"
	"testing"

	apperrors "github.com/jsolteam/tenex-platform/internal/platform/errors"
)

// ── Именованные конструкторы ──────────────────────────────────────────────

func TestDB_SetsCorrectCode(t *testing.T) {
	err := apperrors.DB("userrepo.GetByID", errors.New("connection reset"))
	if err.Code != apperrors.ErrDBQuery {
		t.Errorf("Code = %q, want ErrDBQuery", err.Code)
	}
	if err.Module != "userrepo.GetByID" {
		t.Errorf("Module = %q, want userrepo.GetByID", err.Module)
	}
	if err.Severity != apperrors.SeverityError {
		t.Errorf("Severity = %q, want error", err.Severity)
	}
}

func TestDBConnect_SetsCorrectCode(t *testing.T) {
	err := apperrors.DBConnect("database.Open", errors.New("refused"))
	if err.Code != apperrors.ErrDBConnect {
		t.Errorf("Code = %q, want ErrDBConnect", err.Code)
	}
}

func TestRedis_SetsCorrectCode(t *testing.T) {
	err := apperrors.Redis("cache.Set", errors.New("READONLY"))
	if err.Code != apperrors.ErrRedisUnavailable {
		t.Errorf("Code = %q, want ErrRedisUnavailable", err.Code)
	}
}

func TestS3_SetsCorrectCode(t *testing.T) {
	err := apperrors.S3("mediarepo.Upload", errors.New("NoSuchBucket"))
	if err.Code != apperrors.ErrS3 {
		t.Errorf("Code = %q, want ErrS3", err.Code)
	}
}

func TestExternal_SetsCorrectCode(t *testing.T) {
	err := apperrors.External("telegram.SendMessage", errors.New("429"))
	if err.Code != apperrors.ErrMessengerAPI {
		t.Errorf("Code = %q, want ErrMessengerAPI", err.Code)
	}
}

func TestValidation_SetsWarnSeverity(t *testing.T) {
	cause := errors.New("contact already exists")
	err := apperrors.Validation("userrepo.AddContact", cause)
	if err.Code != apperrors.ErrValidation {
		t.Errorf("Code = %q, want ErrValidation", err.Code)
	}
	if err.Severity != apperrors.SeverityWarn {
		t.Errorf("Severity = %q, want warn", err.Severity)
	}
	if !errors.Is(err, cause) {
		t.Error("errors.Is should find original cause")
	}
}

func TestInternal_SetsErrorSeverity(t *testing.T) {
	err := apperrors.Internal("scheduler.tick", errors.New("nil ptr"))
	if err.Code != apperrors.ErrInternal {
		t.Errorf("Code = %q, want ErrInternal", err.Code)
	}
	if err.Severity != apperrors.SeverityError {
		t.Errorf("Severity = %q, want error", err.Severity)
	}
}

func TestTimeout_IsRetryableByDefault(t *testing.T) {
	err := apperrors.Timeout("http.Do", errors.New("context deadline exceeded"))
	if !err.IsRetryable() {
		t.Error("Timeout should be retryable by default")
	}
	if err.Severity != apperrors.SeverityWarn {
		t.Errorf("Severity = %q, want warn", err.Severity)
	}
}

func TestPanic_SetsFatalSeverity(t *testing.T) {
	err := apperrors.Panic("worker.process", errors.New("nil deref"))
	if err.Code != apperrors.ErrPanic {
		t.Errorf("Code = %q, want ErrPanic", err.Code)
	}
	if err.Severity != apperrors.SeverityFatal {
		t.Errorf("Severity = %q, want fatal", err.Severity)
	}
}

// ── Fluent-цепочка ────────────────────────────────────────────────────────

func TestRetryable_SetsFlag(t *testing.T) {
	err := apperrors.DB("repo", errors.New("deadlock")).Retryable()
	if !err.IsRetryable() {
		t.Error("Retryable() should set retryable flag")
	}
	if err.Severity != apperrors.SeverityWarn {
		t.Errorf("Severity = %q, want warn after Retryable()", err.Severity)
	}
}

func TestRetryable_Idempotent(t *testing.T) {
	err := apperrors.DB("repo", errors.New("x")).Retryable().Retryable()
	if !err.IsRetryable() {
		t.Error("double Retryable() should still be retryable")
	}
}

func TestFatal_SetsSeverity(t *testing.T) {
	err := apperrors.Internal("worker", errors.New("unrecoverable")).Fatal()
	if err.Severity != apperrors.SeverityFatal {
		t.Errorf("Severity = %q, want fatal", err.Severity)
	}
}

func TestInfo_SetsSeverity(t *testing.T) {
	err := apperrors.DB("repo", errors.New("not found")).Info()
	if err.Severity != apperrors.SeverityInfo {
		t.Errorf("Severity = %q, want info", err.Severity)
	}
}

func TestWith_AttrsAppendedInOrder(t *testing.T) {
	err := apperrors.DB("repo.List", errors.New("err")).
		With("user_id", int64(42)).
		With("limit", 100)

	attrs := err.Attrs()
	if len(attrs) != 2 {
		t.Fatalf("expected 2 attrs, got %d", len(attrs))
	}
	if attrs[0].Key != "user_id" || attrs[0].Val != int64(42) {
		t.Errorf("attr[0] = {%s=%v}, want {user_id=42}", attrs[0].Key, attrs[0].Val)
	}
	if attrs[1].Key != "limit" || attrs[1].Val != 100 {
		t.Errorf("attr[1] = {%s=%v}, want {limit=100}", attrs[1].Key, attrs[1].Val)
	}
}

func TestWith_AttrsAppearsInErrorString(t *testing.T) {
	err := apperrors.DB("userrepo", errors.New("timeout")).With("user_id", 99)
	msg := err.Error()
	if !strings.Contains(msg, "user_id=99") {
		t.Errorf("Error() should contain 'user_id=99': %s", msg)
	}
}

func TestFluentChain_AllMethods(t *testing.T) {
	cause := errors.New("root cause")
	err := apperrors.DB("repo", cause).
		Retryable().
		With("table", "reminders").
		With("rows", 500)

	if err.Code != apperrors.ErrDBQuery {
		t.Errorf("Code = %q", err.Code)
	}
	if !err.IsRetryable() {
		t.Error("should be retryable")
	}
	if len(err.Attrs()) != 2 {
		t.Errorf("expected 2 attrs, got %d", len(err.Attrs()))
	}
	if !errors.Is(err, cause) {
		t.Error("errors.Is should find cause")
	}
}

// ── Error() и Unwrap() ────────────────────────────────────────────────────

func TestError_ContainsModuleAndCode(t *testing.T) {
	err := apperrors.DB("userrepo.GetByID", errors.New("disk full"))
	msg := err.Error()
	for _, want := range []string{"userrepo.GetByID", "DB_QUERY", "disk full"} {
		if !strings.Contains(msg, want) {
			t.Errorf("Error() missing %q: %s", want, msg)
		}
	}
}

func TestError_NilCauseNoPanic(t *testing.T) {
	err := apperrors.DB("repo", nil)
	msg := err.Error()
	if !strings.Contains(msg, "DB_QUERY") {
		t.Errorf("Error() missing code: %s", msg)
	}
}

func TestUnwrap_FindsOriginalCause(t *testing.T) {
	root := errors.New("root")
	err := apperrors.DB("repo", root)
	if !errors.Is(err, root) {
		t.Error("errors.Is should find root cause via Unwrap()")
	}
}

func TestUnwrap_WorksThroughStdWrap(t *testing.T) {
	root := errors.New("root")
	appErr := apperrors.DB("repo", root)
	wrapped := errors.Join(appErr, errors.New("other"))
	if !errors.Is(wrapped, root) {
		t.Error("errors.Is should find root through joined error")
	}
}

// ── Глобальные helper-функции ─────────────────────────────────────────────

func TestCodeOf_ReturnsCode(t *testing.T) {
	err := apperrors.DB("repo", errors.New("x"))
	if got := apperrors.CodeOf(err); got != apperrors.ErrDBQuery {
		t.Errorf("CodeOf = %q, want ErrDBQuery", got)
	}
}

func TestCodeOf_PlainErrorReturnsEmpty(t *testing.T) {
	if got := apperrors.CodeOf(errors.New("plain")); got != "" {
		t.Errorf("CodeOf(plain) = %q, want empty", got)
	}
}

func TestCodeOf_NilReturnsEmpty(t *testing.T) {
	if got := apperrors.CodeOf(nil); got != "" {
		t.Errorf("CodeOf(nil) = %q, want empty", got)
	}
}

func TestIsRetryable_RetryableError(t *testing.T) {
	err := apperrors.DB("repo", errors.New("x")).Retryable()
	if !apperrors.IsRetryable(err) {
		t.Error("IsRetryable should return true for retryable error")
	}
}

func TestIsRetryable_NonRetryableError(t *testing.T) {
	err := apperrors.DB("repo", errors.New("x"))
	if apperrors.IsRetryable(err) {
		t.Error("IsRetryable should return false for non-retryable error")
	}
}

func TestIsRetryable_PlainErrorReturnsFalse(t *testing.T) {
	if apperrors.IsRetryable(errors.New("plain")) {
		t.Error("IsRetryable(plain) should return false")
	}
}

func TestIsRetryable_NilReturnsFalse(t *testing.T) {
	if apperrors.IsRetryable(nil) {
		t.Error("IsRetryable(nil) should return false")
	}
}

func TestIsTemporary_MatchesIsRetryable(t *testing.T) {
	errs := []error{
		apperrors.DB("repo", errors.New("x")).Retryable(),
		apperrors.DB("repo", errors.New("x")),
		errors.New("plain"),
		nil,
	}
	for _, err := range errs {
		if apperrors.IsTemporary(err) != apperrors.IsRetryable(err) {
			t.Errorf("IsTemporary and IsRetryable diverged for %v", err)
		}
	}
}

// ── Коды уникальны ────────────────────────────────────────────────────────

func TestErrorCodes_AreDistinct(t *testing.T) {
	codes := []apperrors.ErrorCode{
		apperrors.ErrDBConnect,
		apperrors.ErrDBQuery,
		apperrors.ErrRedisUnavailable,
		apperrors.ErrS3,
		apperrors.ErrMessengerAPI,
		apperrors.ErrValidation,
		apperrors.ErrInvalidInput,
		apperrors.ErrTimeout,
		apperrors.ErrContextCanceled,
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

// ── Обратная совместимость (deprecated функции всё ещё работают) ──────────

func TestWrap_BackwardCompat(t *testing.T) {
	cause := errors.New("connection refused")
	err := apperrors.Wrap(cause, apperrors.ErrDBConnect, "database")
	if err.Code != apperrors.ErrDBConnect {
		t.Errorf("Wrap: Code = %q, want ErrDBConnect", err.Code)
	}
	if err.Module != "database" {
		t.Errorf("Wrap: Module = %q, want database", err.Module)
	}
	if !errors.Is(err, cause) {
		t.Error("Wrap: errors.Is should find cause")
	}
}

func TestNewFatal_BackwardCompat(t *testing.T) {
	cause := errors.New("disk full")
	err := apperrors.NewFatal(cause, apperrors.ErrPanic, "storage")
	if err.Severity != apperrors.SeverityFatal {
		t.Errorf("NewFatal: Severity = %q, want fatal", err.Severity)
	}
}

func TestRetryable_BackwardCompat(t *testing.T) {
	err := apperrors.Retryable(errors.New("timeout"), apperrors.ErrTimeout, "http")
	if !apperrors.IsRetryable(err) {
		t.Error("Retryable(): should be retryable")
	}
}
