package database

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	_ "github.com/lib/pq"

	apperrors "github.com/jsolteam/tenex-platform/internal/platform/errors"
)

const (
	defaultMaxOpenConns    = 25
	defaultMaxIdleConns    = 5
	defaultConnMaxLifetime = 5 * time.Minute
	defaultPingTimeout     = 10 * time.Second
)

type Config struct {
	Host     string
	Port     int
	User     string
	Password string
	Name     string
	SSLMode  string
}

func (c Config) DSN() string {
	return fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		c.Host, c.Port, c.User, c.Password, c.Name, c.SSLMode,
	)
}

type DB struct {
	*sql.DB
}

func (d *DB) SQL() *sql.DB {
	return d.DB
}

func Open(ctx context.Context, cfg Config) (*sql.DB, error) {
	db, err := sql.Open("postgres", cfg.DSN())
	if err != nil {
		return nil, apperrors.DBConnect("database.Open", err)
	}

	db.SetMaxOpenConns(defaultMaxOpenConns)
	db.SetMaxIdleConns(defaultMaxIdleConns)
	db.SetConnMaxLifetime(defaultConnMaxLifetime)

	pingCtx, cancel := context.WithTimeout(ctx, defaultPingTimeout)
	defer cancel()

	if err := db.PingContext(pingCtx); err != nil {
		_ = db.Close()
		return nil, apperrors.DBConnect("database.Open.ping", err)
	}
	return db, nil
}

func OpenAndMigrate(ctx context.Context, cfg Config) (*sql.DB, error) {
	db, err := Open(ctx, cfg)
	if err != nil {
		return nil, err
	}
	if err := New(db).Up(ctx); err != nil {
		_ = db.Close()
		return nil, apperrors.DBConnect("database.OpenAndMigrate.migrate", err)
	}
	return db, nil
}
