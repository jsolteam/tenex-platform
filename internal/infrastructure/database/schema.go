package database

import (
	"database/sql"
	"embed"

	"github.com/jsolteam/tenex-platform/internal/infrastructure/database/migrator"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

func New(db *sql.DB, opts ...migrator.Option) *migrator.Migrator {
	return migrator.New(db, migrationsFS, "migrations", opts...)
}
