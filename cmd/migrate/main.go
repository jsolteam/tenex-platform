package main

import (
	"bufio"
	"context"
	"database/sql"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/jsolteam/tenex-platform/internal/infrastructure/database"
	"github.com/jsolteam/tenex-platform/internal/infrastructure/database/migrator"
	_ "github.com/lib/pq"
)

const usage = `Usage: migrate [flags] <command>

Commands:
  up       Apply all pending migrations
  status   Show migration status
  validate Check that applied migrations match files on disk

Flags:
`

func main() {
	fs := flag.NewFlagSet("migrate", flag.ExitOnError)
	fs.Usage = func() {
		fmt.Fprint(os.Stderr, usage)
		fs.PrintDefaults()
	}

	timeout := fs.Duration("timeout", 60*time.Second, "migration timeout")
	envFile := fs.String("env-file", ".env", "path to .env file (used when APP_ENV != prod)")

	if err := fs.Parse(os.Args[1:]); err != nil {
		os.Exit(1)
	}

	cmd := fs.Arg(0)
	if cmd == "" {
		fs.Usage()
		os.Exit(1)
	}

	appEnv := os.Getenv("APP_ENV")
	isProd := appEnv == "prod" || appEnv == "production"

	if !isProd {
		if err := loadEnvFile(*envFile); err != nil && !os.IsNotExist(err) {
			fmt.Fprintf(os.Stderr, "warn: could not load %s: %v\n", *envFile, err)
		}
	}

	dsn := buildDSN()
	if dsn == "" {
		fmt.Fprintln(os.Stderr, "error: DB_HOST is required")
		fmt.Fprintln(os.Stderr, "  set via environment variables or .env file:")
		fmt.Fprintln(os.Stderr, "  DB_HOST, DB_PORT, DB_USER, DB_PASS, DB_NAME, DB_SSL_MODE")
		os.Exit(1)
	}

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: open db: %v\n", err)
		os.Exit(1)
	}
	defer db.Close()

	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()

	m := database.New(db)

	switch cmd {
	case "up":
		fmt.Printf("[migrate] env=%s, running migrations...\n", appEnv)
		if err := m.Up(ctx); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("[migrate] done.")

	case "status":
		entries, err := m.Status(ctx)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		printStatus(entries)

	case "validate":
		if err := m.Validate(ctx); err != nil {
			fmt.Fprintf(os.Stderr, "validation failed:\n%v\n", err)
			os.Exit(1)
		}
		fmt.Println("ok: all applied migrations match files on disk.")

	default:
		fmt.Fprintf(os.Stderr, "unknown command %q\n", cmd)
		fs.Usage()
		os.Exit(1)
	}
}

func loadEnvFile(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		idx := strings.IndexByte(line, '=')
		if idx < 0 {
			continue
		}

		key := strings.TrimSpace(line[:idx])
		val := strings.TrimSpace(line[idx+1:])

		if i := strings.Index(val, " #"); i >= 0 {
			val = strings.TrimSpace(val[:i])
		}

		if os.Getenv(key) == "" {
			_ = os.Setenv(key, val)
		}
	}
	return scanner.Err()
}

func buildDSN() string {
	host := os.Getenv("DB_HOST")
	if host == "" {
		return ""
	}
	return fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		host,
		envOr("DB_PORT", "5432"),
		envOr("DB_USER", "tenex"),
		envOr("DB_PASS", "tenex"),
		envOr("DB_NAME", "tenex"),
		envOr("DB_SSL_MODE", "disable"),
	)
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func printStatus(entries []migrator.StatusEntry) {
	fmt.Printf("%-6s %-40s %-10s %-12s %s\n", "VER", "NAME", "STATUS", "DURATION", "APPLIED AT")
	fmt.Println(strings.Repeat("─", 80))
	for _, e := range entries {
		status := "pending"
		appliedAt := ""
		duration := ""
		if e.Applied {
			status = "applied"
			appliedAt = e.AppliedAt.Format(time.RFC3339)
			duration = fmt.Sprintf("%dms", e.DurationMs)
			if e.ChecksumMismatch {
				status = "MISMATCH!"
			}
		}
		fmt.Printf("%-6d %-40s %-10s %-12s %s\n", e.Version, e.Name, status, duration, appliedAt)
	}
}
