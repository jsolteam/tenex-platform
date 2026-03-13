package providers_unit

import (
	"strings"
	"testing"

	"github.com/jsolteam/tenex-platform/internal/platform/config/providers"
)

func TestEnvProvider_Name(t *testing.T) {
	p := &providers.EnvProvider{}
	if p.Name() == "" {
		t.Error("Name() should not be empty")
	}
}

func TestEnvProvider_IsRequired(t *testing.T) {
	p := &providers.EnvProvider{}
	type requiredChecker interface{ IsRequired() bool }
	rc, ok := interface{}(p).(requiredChecker)
	if !ok {
		t.Fatal("EnvProvider must implement IsRequired() — C-5 fix missing")
	}
	if !rc.IsRequired() {
		t.Error("EnvProvider.IsRequired() should return true")
	}
}

func TestEnvProvider_NeverErrors(t *testing.T) {
	p := &providers.EnvProvider{}
	_, err := p.Load()
	if err != nil {
		t.Errorf("EnvProvider.Load() should never error, got: %v", err)
	}
}

func TestEnvProvider_ReadsEnvVars(t *testing.T) {
	t.Setenv("DB_HOST", "dbhost-test")
	t.Setenv("DB_PORT", "5433")
	t.Setenv("APP_ENV", "staging")
	t.Setenv("REDIS_ADDR", "redis-test:6380")

	p := &providers.EnvProvider{}
	result, err := p.Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	cases := map[string]string{
		"db.host":    "dbhost-test",
		"db.port":    "5433",
		"app.env":    "staging",
		"redis.addr": "redis-test:6380",
	}
	for key, want := range cases {
		got, ok := result[key]
		if !ok {
			t.Errorf("key %q not in result", key)
			continue
		}
		if got != want {
			t.Errorf("key %q = %q, want %q", key, got, want)
		}
	}
}

func TestEnvProvider_EmptyVarsExcluded(t *testing.T) {
	t.Setenv("APP_NAME", "")

	p := &providers.EnvProvider{}
	result, err := p.Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if _, ok := result["app.name"]; ok {
		t.Error("empty env var should not be included in result")
	}
}

func TestEnvProvider_UnsetVarsExcluded(t *testing.T) {
	p := &providers.EnvProvider{}
	result, _ := p.Load()
	if v, ok := result["clients.telegram.token"]; ok && v == "" {
		t.Error("empty token should not be included")
	}
}

func TestDBProvider_Name(t *testing.T) {
	p := &providers.DBProvider{}
	if p.Name() == "" {
		t.Error("DBProvider.Name() should not be empty")
	}
}

func TestDBProvider_LoadReturnsError(t *testing.T) {
	p := &providers.DBProvider{}
	result, err := p.Load()

	if err == nil {
		t.Error("DBProvider.Load() must return error — prevents silent config override")
	}
	if result != nil {
		t.Error("DBProvider.Load() should return nil map on error")
	}
	if !strings.Contains(err.Error(), "not implemented") {
		t.Errorf("error should say 'not implemented', got: %v", err)
	}
}

// DBProvider должен быть опциональным — чтобы не роняло старт до реализации.
func TestDBProvider_IsOptional(t *testing.T) {
	p := &providers.DBProvider{}
	type requiredChecker interface{ IsRequired() bool }
	if rc, ok := interface{}(p).(requiredChecker); ok {
		if rc.IsRequired() {
			t.Error("DBProvider should be optional (IsRequired=false) — not yet implemented")
		}
	}
}
