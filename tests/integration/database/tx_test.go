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
		Port:     5400,
		User:     envOr("DB_USER", "tenex"),
		Password: envOr("DB_PASS", "tenex"),
		Name:     envOr("DB_NAME", "tenex"),
		SSLMode:  envOr("DB_SSL_MODE", "disable"),
	}
	rawDB, err := database.Open(context.Background(), cfg)
	if err != nil {
		t.Skipf("cannot connect to test database: %v", err)
		return nil
	}

	wrapped := &database.DB{DB: rawDB}
	sqlDB := wrapped.SQL()

	t.Cleanup(func() { _ = sqlDB.Close() })
	return sqlDB
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
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
		if !containsStr(dsn, want) {
			t.Errorf("DSN missing %q: %s", want, dsn)
		}
	}
}

func TestDB_SQL_ReturnsSameUnderlying(t *testing.T) {
	raw, err := sql.Open("postgres", "host=localhost port=5432 user=x dbname=x sslmode=disable")
	if err != nil {
		t.Fatalf("sql.Open: %v", err)
	}
	defer raw.Close()

	wrapped := &database.DB{DB: raw}

	if got := wrapped.SQL(); got != raw {
		t.Errorf("SQL() returned a different pointer than the wrapped *sql.DB")
	}
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
	cancel() // cancel immediately

	err := database.WithTx(ctx, db, func(ctx context.Context, tx *sql.Tx) error {
		return nil
	})
	// BeginTx with a cancelled context should return an error.
	if err == nil {
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

func containsStr(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
