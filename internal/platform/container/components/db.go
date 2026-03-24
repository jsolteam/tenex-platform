package components

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	_ "github.com/lib/pq"

	"github.com/jsolteam/tenex-platform/internal/infrastructure/database"
)

type DBComponent struct {
	cfg *ConfigComponent
	db  *database.DB
}

func NewDB(cfg *ConfigComponent) *DBComponent {
	return &DBComponent{cfg: cfg}
}

func (d *DBComponent) Start(ctx context.Context) error {
	appCfg := d.cfg.Get()

	cfg := database.Config{
		Host:     appCfg.DB.Host,
		Port:     appCfg.DB.Port,
		User:     appCfg.DB.User,
		Password: appCfg.DB.Password,
		Name:     appCfg.DB.Name,
		SSLMode:  appCfg.DB.SSLMode,
	}

	sqlDB, err := database.Open(ctx, cfg)
	if err != nil {
		return fmt.Errorf("db component: open: %w", err)
	}

	migrateCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()

	if err := database.New(sqlDB).Up(migrateCtx); err != nil {
		_ = sqlDB.Close()
		return fmt.Errorf("db component: migrate: %w", err)
	}

	d.db = &database.DB{DB: sqlDB}
	return nil
}

func (d *DBComponent) Stop(_ context.Context) error {
	if d.db != nil {
		return d.db.Close()
	}
	return nil
}

func (d *DBComponent) DB() *database.DB {
	if d.db == nil {
		panic("DBComponent.DB() called before Start()")
	}
	return d.db
}

func (d *DBComponent) SQL() *sql.DB {
	if d.db == nil {
		panic("DBComponent.SQL() called before Start()")
	}
	return d.db.SQL()
}
