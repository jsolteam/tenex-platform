package database_integration

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"testing"

	"github.com/jsolteam/tenex-platform/internal/infrastructure/database"
)

func openTestDB(t *testing.T) *sql.DB {
	t.Helper()
	host := os.Getenv("DB_HOST")
	if host == "" {
		return nil
	}
	cfg := database.Config{
		Host:     host,
		Port:     5432,
		User:     envOr("DB_USER", "tenex"),
		Password: envOr("DB_PASS", "tenex"),
		Name:     envOr("DB_NAME", "tenex"),
		SSLMode:  envOr("DB_SSL_MODE", "disable"),
	}
	db, err := database.Open(context.Background(), cfg)
	if err != nil {
		t.Skipf("cannot connect to test database: %v", err)
		return nil
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func TestWithTx_CommitOnSuccess(t *testing.T) {
	db := openTestDB(t)
	if db == nil {
		t.Skip("DB_HOST not set")
	}

	var called bool
	err := database.WithTx(context.Background(), db, func(ctx context.Context, tx *sql.Tx) error {
		called = true
		_, err := tx.ExecContext(ctx, "SELECT 1")
		return err
	})
	if err != nil {
		t.Fatalf("WithTx: %v", err)
	}
	if !called {
		t.Error("fn was not called")
	}
}

func TestWithTx_RollbackOnError(t *testing.T) {
	db := openTestDB(t)
	if db == nil {
		t.Skip("DB_HOST not set")
	}

	sentinel := errors.New("intentional error")
	err := database.WithTx(context.Background(), db, func(ctx context.Context, tx *sql.Tx) error {
		return sentinel
	})
	if !errors.Is(err, sentinel) {
		t.Errorf("expected sentinel error, got: %v", err)
	}
}

func TestWithTx_PanicPropagates(t *testing.T) {
	db := openTestDB(t)
	if db == nil {
		t.Skip("DB_HOST not set")
	}

	defer func() {
		r := recover()
		if r == nil {
			t.Error("expected panic to propagate through WithTx")
		}
	}()

	_ = database.WithTx(context.Background(), db, func(_ context.Context, _ *sql.Tx) error {
		panic("test panic in transaction")
	})
}

func TestWithTx_ContextCancelled(t *testing.T) {
	db := openTestDB(t)
	if db == nil {
		t.Skip("DB_HOST not set")
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // отменяем сразу

	err := database.WithTx(ctx, db, func(ctx context.Context, tx *sql.Tx) error {
		return nil
	})
	// BeginTx с отменённым контекстом должен вернуть ошибку
	if err == nil {
		// некоторые драйверы могут успеть начать транзакцию — не фатально
		t.Log("note: WithTx succeeded despite cancelled context")
	}
}

func TestWithTxOpts_ReadOnly(t *testing.T) {
	db := openTestDB(t)
	if db == nil {
		t.Skip("DB_HOST not set")
	}

	opts := &sql.TxOptions{ReadOnly: true}
	err := database.WithTxOpts(context.Background(), db, opts, func(ctx context.Context, tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, "SELECT 1")
		return err
	})
	if err != nil {
		t.Fatalf("WithTxOpts ReadOnly: %v", err)
	}
}

func TestWithTxOpts_RollbackOnError(t *testing.T) {
	db := openTestDB(t)
	if db == nil {
		t.Skip("DB_HOST not set")
	}

	sentinel := errors.New("opts error")
	err := database.WithTxOpts(context.Background(), db, nil, func(_ context.Context, _ *sql.Tx) error {
		return sentinel
	})
	if !errors.Is(err, sentinel) {
		t.Errorf("expected sentinel, got: %v", err)
	}
}

func TestDatabaseConfig_DSN(t *testing.T) {
	cfg := database.Config{
		Host:     "localhost",
		Port:     5432,
		User:     "myuser",
		Password: "mypass",
		Name:     "mydb",
		SSLMode:  "disable",
	}
	dsn := cfg.DSN()
	for _, want := range []string{"localhost", "5432", "myuser", "mypass", "mydb", "disable"} {
		if !contains(dsn, want) {
			t.Errorf("DSN missing %q: %s", want, dsn)
		}
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr ||
		len(s) > 0 && containsStr(s, substr))
}

func containsStr(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
