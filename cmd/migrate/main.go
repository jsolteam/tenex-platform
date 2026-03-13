package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/jsolteam/tenex-platform/internal/infrastructure/database/migrator"
	"github.com/jsolteam/tenex-platform/internal/infrastructure/database/schema"
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

	if err := fs.Parse(os.Args[1:]); err != nil {
		os.Exit(1)
	}

	cmd := fs.Arg(0)
	if cmd == "" {
		fs.Usage()
		os.Exit(1)
	}

	dsn := buildDSN()
	if dsn == "" {
		fmt.Fprintln(os.Stderr, "error: DB_HOST is required (set DB_HOST, DB_PORT, DB_USER, DB_PASS, DB_NAME, DB_SSL_MODE)")
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

	m := schema.New(db)

	switch cmd {
	case "up":
		fmt.Println("running migrations...")
		if err := m.Up(ctx); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("done.")

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

func buildDSN() string {
	host := env("DB_HOST", "")
	if host == "" {
		return ""
	}
	return fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		host,
		env("DB_PORT", "5432"),
		env("DB_USER", "tenex"),
		env("DB_PASS", "tenex"),
		env("DB_NAME", "tenex"),
		env("DB_SSL_MODE", "disable"),
	)
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func printStatus(entries []migrator.StatusEntry) {
	fmt.Printf("%-6s %-40s %-10s %s\n", "VER", "NAME", "STATUS", "APPLIED AT")
	fmt.Println("─────────────────────────────────────────────────────────────────────")
	for _, e := range entries {
		status := "pending"
		appliedAt := ""
		if e.Applied {
			status = "applied"
			appliedAt = e.AppliedAt.Format(time.RFC3339)
			if e.ChecksumMismatch {
				status = "MISMATCH!"
			}
		}
		fmt.Printf("%-6d %-40s %-10s %s\n", e.Version, e.Name, status, appliedAt)
	}
}
