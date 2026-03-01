package config

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

func Validate(cfg *AppConfig) error {
	var errs []error

	if cfg.DB.Host == "" {
		errs = append(errs, errors.New("db.host is required"))
	}
	if cfg.DB.User == "" {
		errs = append(errs, errors.New("db.user is required"))
	}
	if cfg.DB.Name == "" {
		errs = append(errs, errors.New("db.name is required"))
	}
	if cfg.DB.Port <= 0 || cfg.DB.Port > 65535 {
		errs = append(errs, fmt.Errorf("db.port must be in [1, 65535], got %d", cfg.DB.Port))
	}
	if cfg.Redis.Addr == "" {
		errs = append(errs, errors.New("redis.addr is required"))
	}
	if cfg.Scheduler.MaxRetries < 1 {
		errs = append(errs, fmt.Errorf(
			"scheduler.max_retries must be >= 1 (got %d); value 0 causes infinite retry loop",
			cfg.Scheduler.MaxRetries,
		))
	}
	const minRetryInterval = time.Second
	if cfg.Scheduler.ReminderRetryInterval < minRetryInterval {
		errs = append(errs, fmt.Errorf(
			"scheduler.reminder_retry_interval must be >= %s (got %s); smaller values cause busy loop",
			minRetryInterval, cfg.Scheduler.ReminderRetryInterval,
		))
	}

	if lvl := strings.ToLower(cfg.App.LogLevel); lvl != "" {
		validLevels := map[string]bool{"debug": true, "info": true, "warn": true, "error": true}
		if !validLevels[lvl] {
			errs = append(errs, fmt.Errorf(
				"app.log_level must be one of debug|info|warn|error, got %q",
				cfg.App.LogLevel,
			))
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("config validation failed: %w", errors.Join(errs...))
	}
	return nil
}
