package config_unit

import (
	"strings"
	"testing"
	"time"

	"github.com/jsol/tenex-platform/internal/platform/config"
)

func validCfg() *config.AppConfig {
	return &config.AppConfig{
		DB: config.DB{
			Host: "localhost",
			Port: 5432,
			User: "tenex",
			Name: "tenex",
		},
		Redis: config.Redis{Addr: "localhost:6379"},
		Scheduler: config.Scheduler{
			MaxRetries:            3,
			ReminderRetryInterval: 30 * time.Second,
		},
	}
}

func TestValidate_ValidConfig(t *testing.T) {
	if err := config.Validate(validCfg()); err != nil {
		t.Errorf("valid config should not fail: %v", err)
	}
}

func TestValidate_DBHostRequired(t *testing.T) {
	cfg := validCfg()
	cfg.DB.Host = ""
	assertError(t, cfg, "db.host")
}

func TestValidate_DBUserRequired(t *testing.T) {
	cfg := validCfg()
	cfg.DB.User = ""
	assertError(t, cfg, "db.user")
}

func TestValidate_DBNameRequired(t *testing.T) {
	cfg := validCfg()
	cfg.DB.Name = ""
	assertError(t, cfg, "db.name")
}

func TestValidate_DBPortZero(t *testing.T) {
	cfg := validCfg()
	cfg.DB.Port = 0
	assertError(t, cfg, "db.port")
}

func TestValidate_DBPortNegative(t *testing.T) {
	cfg := validCfg()
	cfg.DB.Port = -1
	assertError(t, cfg, "db.port")
}

func TestValidate_DBPortTooHigh(t *testing.T) {
	cfg := validCfg()
	cfg.DB.Port = 65536
	assertError(t, cfg, "db.port")
}

func TestValidate_DBPortBoundaryValid(t *testing.T) {
	cfg := validCfg()
	for _, port := range []int{1, 5432, 65535} {
		cfg.DB.Port = port
		if err := config.Validate(cfg); err != nil {
			t.Errorf("port %d should be valid: %v", port, err)
		}
	}
}

func TestValidate_RedisAddrRequired(t *testing.T) {
	cfg := validCfg()
	cfg.Redis.Addr = ""
	assertError(t, cfg, "redis.addr")
}

func TestValidate_SchedulerMaxRetriesZero(t *testing.T) {
	cfg := validCfg()
	cfg.Scheduler.MaxRetries = 0
	assertError(t, cfg, "max_retries")
}

func TestValidate_SchedulerMaxRetriesNegative(t *testing.T) {
	cfg := validCfg()
	cfg.Scheduler.MaxRetries = -1
	assertError(t, cfg, "max_retries")
}

func TestValidate_SchedulerMaxRetriesOne_Valid(t *testing.T) {
	cfg := validCfg()
	cfg.Scheduler.MaxRetries = 1
	if err := config.Validate(cfg); err != nil {
		t.Errorf("MaxRetries=1 should be valid: %v", err)
	}
}

func TestValidate_SchedulerIntervalZero(t *testing.T) {
	cfg := validCfg()
	cfg.Scheduler.ReminderRetryInterval = 0
	assertError(t, cfg, "reminder_retry_interval")
}

func TestValidate_SchedulerIntervalTooSmall(t *testing.T) {
	cfg := validCfg()
	cfg.Scheduler.ReminderRetryInterval = 500 * time.Millisecond
	assertError(t, cfg, "reminder_retry_interval")
}

func TestValidate_SchedulerIntervalOneSecond_Valid(t *testing.T) {
	cfg := validCfg()
	cfg.Scheduler.ReminderRetryInterval = time.Second
	if err := config.Validate(cfg); err != nil {
		t.Errorf("ReminderRetryInterval=1s should be valid: %v", err)
	}
}

func TestValidate_MultipleErrors(t *testing.T) {
	cfg := validCfg()
	cfg.DB.Host = ""
	cfg.DB.User = ""
	cfg.Redis.Addr = ""

	err := config.Validate(cfg)
	if err == nil {
		t.Fatal("expected validation error, got nil")
	}
	for _, want := range []string{"db.host", "db.user", "redis.addr"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error should contain %q, got: %v", want, err)
		}
	}
}

func assertError(t *testing.T, cfg *config.AppConfig, wantSubstr string) {
	t.Helper()
	err := config.Validate(cfg)
	if err == nil {
		t.Fatalf("expected error containing %q, got nil", wantSubstr)
	}
	if !strings.Contains(err.Error(), wantSubstr) {
		t.Errorf("expected error to contain %q, got: %v", wantSubstr, err)
	}
}
