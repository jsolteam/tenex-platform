package migrator

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

type Migration struct {
	Version  int
	Name     string
	Filename string
	SQL      string
	Checksum string
}

type AppliedMigration struct {
	Version    int
	Name       string
	Checksum   string
	AppliedAt  time.Time
	DurationMs int64
}

type Migrator struct {
	db     *sql.DB
	fsys   embed.FS
	dir    string
	table  string
	lockID int64
}

type Option func(*Migrator)

func WithTable(name string) Option {
	return func(m *Migrator) { m.table = name }
}

func WithLockID(id int64) Option {
	return func(m *Migrator) { m.lockID = id }
}

func New(db *sql.DB, fsys embed.FS, dir string, opts ...Option) *Migrator {
	m := &Migrator{
		db:     db,
		fsys:   fsys,
		dir:    dir,
		table:  "schema_migrations",
		lockID: 0x54454E4558, // "TENEX" in hex — unique advisory lock
	}
	for _, o := range opts {
		o(m)
	}
	return m
}

func (m *Migrator) Up(ctx context.Context) error {
	if err := m.ensureTable(ctx); err != nil {
		return fmt.Errorf("migrator: ensure table: %w", err)
	}

	if err := m.acquireLock(ctx); err != nil {
		return fmt.Errorf("migrator: acquire lock: %w", err)
	}
	defer m.releaseLock(ctx) //nolint:errcheck

	pending, err := m.pending(ctx)
	if err != nil {
		return fmt.Errorf("migrator: load pending: %w", err)
	}

	if len(pending) == 0 {
		return nil
	}

	for _, mig := range pending {
		if err := m.apply(ctx, mig); err != nil {
			return fmt.Errorf("migrator: apply v%04d (%s): %w", mig.Version, mig.Name, err)
		}
	}

	return nil
}

func (m *Migrator) Status(ctx context.Context) ([]StatusEntry, error) {
	if err := m.ensureTable(ctx); err != nil {
		return nil, fmt.Errorf("migrator: ensure table: %w", err)
	}

	all, err := m.load()
	if err != nil {
		return nil, err
	}

	applied, err := m.applied(ctx)
	if err != nil {
		return nil, err
	}

	appliedMap := make(map[int]AppliedMigration, len(applied))
	for _, a := range applied {
		appliedMap[a.Version] = a
	}

	entries := make([]StatusEntry, 0, len(all))
	for _, mig := range all {
		e := StatusEntry{Version: mig.Version, Name: mig.Name}
		if a, ok := appliedMap[mig.Version]; ok {
			e.Applied = true
			e.AppliedAt = a.AppliedAt
			e.DurationMs = a.DurationMs
			if a.Checksum != mig.Checksum {
				e.ChecksumMismatch = true
			}
		}
		entries = append(entries, e)
	}
	return entries, nil
}

func (m *Migrator) Validate(ctx context.Context) error {
	entries, err := m.Status(ctx)
	if err != nil {
		return err
	}

	var mismatches []string
	for _, e := range entries {
		if e.Applied && e.ChecksumMismatch {
			mismatches = append(mismatches,
				fmt.Sprintf("v%04d (%s): checksum mismatch — migration file was modified after being applied", e.Version, e.Name),
			)
		}
	}
	if len(mismatches) > 0 {
		return fmt.Errorf("migrator: validate failed:\n  %s", strings.Join(mismatches, "\n  "))
	}
	return nil
}

type StatusEntry struct {
	Version          int
	Name             string
	Applied          bool
	AppliedAt        time.Time
	DurationMs       int64
	ChecksumMismatch bool
}

// internal helpers
func (m *Migrator) ensureTable(ctx context.Context) error {
	_, err := m.db.ExecContext(ctx, fmt.Sprintf(`
		CREATE TABLE IF NOT EXISTS %s (
			version      INT         NOT NULL PRIMARY KEY,
			name         TEXT        NOT NULL,
			checksum     TEXT        NOT NULL,
			applied_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
			duration_ms  BIGINT      NOT NULL DEFAULT 0
		)`, m.table))
	return err
}

func (m *Migrator) acquireLock(ctx context.Context) error {
	var ok bool
	err := m.db.QueryRowContext(ctx,
		`SELECT pg_try_advisory_lock($1)`, m.lockID,
	).Scan(&ok)
	if err != nil {
		return err
	}
	if !ok {
		return errors.New("another migration process is already running (advisory lock held)")
	}
	return nil
}

func (m *Migrator) releaseLock(ctx context.Context) error {
	_, err := m.db.ExecContext(ctx, `SELECT pg_advisory_unlock($1)`, m.lockID)
	return err
}

func (m *Migrator) load() ([]Migration, error) {
	entries, err := fs.ReadDir(m.fsys, m.dir)
	if err != nil {
		return nil, fmt.Errorf("read migrations dir %q: %w", m.dir, err)
	}

	var migrations []Migration
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".sql" {
			continue
		}

		version, name, err := parseFilename(e.Name())
		if err != nil {
			return nil, fmt.Errorf("parse filename %q: %w", e.Name(), err)
		}

		content, err := m.fsys.ReadFile(m.dir + "/" + e.Name())
		if err != nil {
			return nil, fmt.Errorf("read %q: %w", e.Name(), err)
		}

		migrations = append(migrations, Migration{
			Version:  version,
			Name:     name,
			Filename: e.Name(),
			SQL:      string(content),
			Checksum: checksum(content),
		})
	}

	sort.Slice(migrations, func(i, j int) bool {
		return migrations[i].Version < migrations[j].Version
	})

	if err := detectDuplicates(migrations); err != nil {
		return nil, err
	}

	return migrations, nil
}

func (m *Migrator) applied(ctx context.Context) ([]AppliedMigration, error) {
	rows, err := m.db.QueryContext(ctx,
		fmt.Sprintf(`SELECT version, name, checksum, applied_at, duration_ms FROM %s ORDER BY version`, m.table))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []AppliedMigration
	for rows.Next() {
		var a AppliedMigration
		if err := rows.Scan(&a.Version, &a.Name, &a.Checksum, &a.AppliedAt, &a.DurationMs); err != nil {
			return nil, err
		}
		result = append(result, a)
	}
	return result, rows.Err()
}

func (m *Migrator) pending(ctx context.Context) ([]Migration, error) {
	all, err := m.load()
	if err != nil {
		return nil, err
	}

	applied, err := m.applied(ctx)
	if err != nil {
		return nil, err
	}

	appliedSet := make(map[int]struct{}, len(applied))
	for _, a := range applied {
		appliedSet[a.Version] = struct{}{}
	}

	var pending []Migration
	for _, mig := range all {
		if _, ok := appliedSet[mig.Version]; !ok {
			pending = append(pending, mig)
		}
	}
	return pending, nil
}

func (m *Migrator) apply(ctx context.Context, mig Migration) error {
	tx, err := m.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	start := time.Now()

	if _, err = tx.ExecContext(ctx, mig.SQL); err != nil {
		return fmt.Errorf("exec SQL: %w", err)
	}

	durationMs := time.Since(start).Milliseconds()

	_, err = tx.ExecContext(ctx, fmt.Sprintf(`
		INSERT INTO %s (version, name, checksum, applied_at, duration_ms)
		VALUES ($1, $2, $3, now(), $4)`, m.table),
		mig.Version, mig.Name, mig.Checksum, durationMs,
	)
	if err != nil {
		return fmt.Errorf("record migration: %w", err)
	}

	return tx.Commit()
}

func parseFilename(name string) (version int, migName string, err error) {
	base := strings.TrimSuffix(name, ".sql")
	parts := strings.SplitN(base, "_", 2)
	if len(parts) != 2 {
		return 0, "", fmt.Errorf("expected format NNNN_name.sql, got %q", name)
	}

	v, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, "", fmt.Errorf("version prefix is not an integer in %q", name)
	}
	if v <= 0 {
		return 0, "", fmt.Errorf("version must be > 0 in %q", name)
	}

	return v, parts[1], nil
}

func checksum(content []byte) string {
	h := sha256.Sum256(content)
	return fmt.Sprintf("%x", h)
}

func detectDuplicates(migrations []Migration) error {
	seen := make(map[int]string, len(migrations))
	for _, m := range migrations {
		if prev, ok := seen[m.Version]; ok {
			return fmt.Errorf("duplicate migration version %d: files %q and %q", m.Version, prev, m.Filename)
		}
		seen[m.Version] = m.Filename
	}
	return nil
}
