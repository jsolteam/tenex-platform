package config_unit

import (
	"errors"
	"strings"
	"testing"

	"github.com/jsolteam/tenex-platform/internal/platform/config"
)

type requiredStub struct {
	name string
	data map[string]interface{}
	err  error
}

func (r *requiredStub) Name() string                          { return r.name }
func (r *requiredStub) IsRequired() bool                      { return true }
func (r *requiredStub) Load() (map[string]interface{}, error) { return r.data, r.err }

type optionalStub struct {
	name string
	data map[string]interface{}
	err  error
}

func (o *optionalStub) Name() string                          { return o.name }
func (o *optionalStub) IsRequired() bool                      { return false }
func (o *optionalStub) Load() (map[string]interface{}, error) { return o.data, o.err }

type plainStub struct {
	name string
	data map[string]interface{}
}

func (p *plainStub) Name() string                          { return p.name }
func (p *plainStub) Load() (map[string]interface{}, error) { return p.data, nil }

// минимально корректный набор ключей
func minSettings() map[string]interface{} {
	return map[string]interface{}{
		"db.host":                           "localhost",
		"db.port":                           "5432",
		"db.user":                           "tenex",
		"db.name":                           "tenex",
		"redis.addr":                        "localhost:6379",
		"scheduler.max_retries":             "3",
		"scheduler.reminder_retry_interval": "30s",
	}
}

// TestLoader_RequiredProviderFailure — если required-провайдер падает,
// Load() возвращает ошибку со словом "required".
func TestLoader_RequiredProviderFailure(t *testing.T) {
	p := &requiredStub{name: "req", err: errors.New("connection refused")}
	loader := config.NewLoader(p)

	_, err := loader.Load()
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "required") {
		t.Errorf("error should mention 'required', got: %v", err)
	}
}

// TestLoader_OptionalProviderFailure — опциональный провайдер:
// ошибка пишется в stderr, Load() продолжает работу.
func TestLoader_OptionalProviderFailure(t *testing.T) {
	req := &requiredStub{name: "req", data: minSettings()}
	opt := &optionalStub{name: "opt", err: errors.New("file not found")}

	loader := config.NewLoader(req, opt)
	cfg, err := loader.Load()
	if err != nil {
		t.Fatalf("optional failure should not cause error, got: %v", err)
	}
	if cfg == nil {
		t.Fatal("config is nil after optional provider failure")
	}
}

// TestLoader_OverrideOrder — более поздний провайдер перезаписывает ранний.
func TestLoader_OverrideOrder(t *testing.T) {
	base := &plainStub{name: "base", data: minSettings()}
	override := &plainStub{name: "override", data: map[string]interface{}{
		"app.env": "staging",
	}}

	loader := config.NewLoader(base, override)
	cfg, err := loader.Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if cfg.App.Env != "staging" {
		t.Errorf("App.Env = %q, want %q", cfg.App.Env, "staging")
	}
}

// TestLoader_EmptyStringSkipped — пустая строка не перезаписывает дефолт.
func TestLoader_EmptyStringSkipped(t *testing.T) {
	s := minSettings()
	s["app.env"] = "" // должна быть проигнорирована
	loader := config.NewLoader(&plainStub{name: "p", data: s})

	cfg, err := loader.Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	// Дефолт app.env = "local" — пустая строка не должна его затереть.
	if cfg.App.Env == "" {
		t.Error("empty string overwrote default — isEmptyValue broken")
	}
}

// TestLoader_DefaultsApplied — дефолты из defaults.go применяются.
func TestLoader_DefaultsApplied(t *testing.T) {
	loader := config.NewLoader(&plainStub{name: "p", data: minSettings()})
	cfg, err := loader.Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if cfg.App.Name == "" {
		t.Error("App.Name default not applied")
	}
	if cfg.DB.SSLMode == "" {
		t.Error("DB.SSLMode default not applied")
	}
}

// TestLoader_ValidationCalledOnLoad — если валидация не проходит → ошибка.
func TestLoader_ValidationCalledOnLoad(t *testing.T) {
	s := minSettings()
	delete(s, "db.user") // нет дефолта → Viper вернёт "" → validator: "db.user is required"
	loader := config.NewLoader(&plainStub{name: "p", data: s})

	_, err := loader.Load()
	if err == nil {
		t.Fatal("expected validation error, got nil")
	}
	if !strings.Contains(err.Error(), "db.user") {
		t.Errorf("error should mention db.user, got: %v", err)
	}
}
