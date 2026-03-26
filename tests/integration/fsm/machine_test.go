package fsm_integration

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/jsolteam/tenex-platform/internal/fsm"
	infraredis "github.com/jsolteam/tenex-platform/internal/infrastructure/redis"
	"github.com/jsolteam/tenex-platform/internal/platform/logger/core"
	"github.com/jsolteam/tenex-platform/internal/platform/observability/tracing"
)

// setup подключается к Redis и возвращает fsm.Store.
// Пропускает тест если REDIS_ADDR не задан.
func setup(t *testing.T) fsm.Store {
	t.Helper()
	addr := os.Getenv("REDIS_ADDR")
	if addr == "" {
		t.Skip("REDIS_ADDR не задан — пропускаем integration тест FSM")
	}
	cfg := infraredis.Config{
		Addr: addr,
		DB:   13, // отдельная DB для FSM-тестов
	}
	client, err := infraredis.New(context.Background(), cfg, core.NewNoop(), tracing.NewNoop())
	if err != nil {
		t.Fatalf("redis connect: %v", err)
	}
	_ = client.Underlying().FlushDB(context.Background()).Err()
	t.Cleanup(func() {
		_ = client.Underlying().FlushDB(context.Background())
		_ = client.Close()
	})
	return fsm.NewRedisStore(infraredis.NewFSMStore(client))
}

// newMachine создаёт Machine с изолированным реестром для конкретного теста.
func newMachine(store fsm.Store, messenger, userID string) (fsm.Machine, *fsm.Registry) {
	reg := fsm.NewRegistry()
	machine := fsm.NewWithStore(store, messenger, userID, fsm.Options{Registry: reg})
	return machine, reg
}

// registerLinear регистрирует простой линейный сценарий из трёх шагов.
func registerLinear(reg *fsm.Registry, prefix string) (s1, s2, s3 fsm.State) {
	s1 = fsm.NewState(prefix, "step1")
	s2 = fsm.NewState(prefix, "step2")
	s3 = fsm.NewState(prefix, "step3")
	reg.RegisterState(s1, "Шаг 1")
	reg.RegisterState(s2, "Шаг 2")
	reg.RegisterState(s3, "Шаг 3")
	reg.MustRegister(
		fsm.TransitionRule{From: fsm.Idle, To: s1, Description: "начало"},
		fsm.TransitionRule{From: s1, To: s2, Description: "1→2"},
		fsm.TransitionRule{From: s2, To: s3, Description: "2→3"},
	)
	return
}

// TestIntegration_FullScenario проверяет полный сценарий через реальный Redis:
// Idle → step1 → step2 → step3 → Reset → Idle
func TestIntegration_FullScenario(t *testing.T) {
	store := setup(t)
	machine, reg := newMachine(store, "telegram", "user_full")
	step1, step2, step3 := registerLinear(reg, "full")
	ctx := context.Background()

	// Начальное состояние — Idle
	idle, err := machine.IsIdle(ctx)
	if err != nil {
		t.Fatalf("IsIdle: %v", err)
	}
	if !idle {
		t.Fatal("начальное состояние должно быть Idle")
	}

	// Переход в step1 с данными
	d1 := fsm.NewStateData()
	d1.Set("name", "Аспирин")
	if err := machine.Transition(ctx, step1, d1); err != nil {
		t.Fatalf("Transition → step1: %v", err)
	}

	current, err := machine.Current(ctx)
	if err != nil {
		t.Fatalf("Current: %v", err)
	}
	if current.State != step1 {
		t.Errorf("Current.State = %q, ожидали %q", current.State, step1)
	}
	if current.Data.GetString("name") != "Аспирин" {
		t.Error("данные step1 не сохранились в Redis")
	}

	// Переход в step2
	d2 := fsm.NewStateData()
	d2.Set("photo", "s3/photo.jpg")
	if err := machine.Transition(ctx, step2, d2); err != nil {
		t.Fatalf("Transition → step2: %v", err)
	}

	in2, _ := machine.InState(ctx, step2)
	if !in2 {
		t.Error("должны быть в step2")
	}

	// Переход в step3
	if err := machine.Transition(ctx, step3, fsm.NewStateData()); err != nil {
		t.Fatalf("Transition → step3: %v", err)
	}

	// Reset → Idle
	if err := machine.Reset(ctx); err != nil {
		t.Fatalf("Reset: %v", err)
	}

	idle, _ = machine.IsIdle(ctx)
	if !idle {
		t.Error("после Reset должны быть в Idle")
	}
}

// TestIntegration_UserIsolation — состояния разных пользователей независимы.
func TestIntegration_UserIsolation(t *testing.T) {
	store := setup(t)
	machine1, reg := newMachine(store, "telegram", "iso_user1")
	machine2 := fsm.NewWithStore(store, "telegram", "iso_user2", fsm.Options{Registry: reg})

	step1 := fsm.NewState("isolation", "s1")
	reg.RegisterState(step1, "Шаг 1")
	reg.MustRegister(fsm.TransitionRule{From: fsm.Idle, To: step1, Description: "старт"})

	ctx := context.Background()
	_ = machine1.Transition(ctx, step1, fsm.NewStateData())

	idle2, err := machine2.IsIdle(ctx)
	if err != nil {
		t.Fatalf("IsIdle user2: %v", err)
	}
	if !idle2 {
		t.Error("состояние user1 не должно влиять на user2")
	}
}

// TestIntegration_MessengerIsolation — разные мессенджеры изолированы.
func TestIntegration_MessengerIsolation(t *testing.T) {
	store := setup(t)
	machTG, reg := newMachine(store, "telegram", "shared_user")
	machVK := fsm.NewWithStore(store, "vk", "shared_user", fsm.Options{Registry: reg})

	step1 := fsm.NewState("msg_iso", "s1")
	reg.RegisterState(step1, "Шаг 1")
	reg.MustRegister(fsm.TransitionRule{From: fsm.Idle, To: step1, Description: "старт"})

	ctx := context.Background()
	_ = machTG.Transition(ctx, step1, fsm.NewStateData())

	idleVK, _ := machVK.IsIdle(ctx)
	if !idleVK {
		t.Error("telegram не должен влиять на vk для одного userID")
	}
}

// TestIntegration_TTLExpiry — после истечения TTL Current возвращает Idle.
func TestIntegration_TTLExpiry(t *testing.T) {
	if testing.Short() {
		t.Skip("TTL тест пропущен в -short режиме")
	}

	addr := os.Getenv("REDIS_ADDR")
	if addr == "" {
		t.Skip("REDIS_ADDR не задан")
	}

	cfg := infraredis.Config{Addr: addr, DB: 13}
	client, _ := infraredis.New(context.Background(), cfg, core.NewNoop(), tracing.NewNoop())
	t.Cleanup(func() { _ = client.Close() })

	store := fsm.NewRedisStore(infraredis.NewFSMStore(client))
	reg := fsm.NewRegistry()

	step1 := fsm.NewState("ttl_test", "s1")
	reg.RegisterState(step1, "Шаг 1")
	reg.MustRegister(fsm.TransitionRule{From: fsm.Idle, To: step1, Description: "старт"})

	machine := fsm.NewWithStore(store, "telegram", "user_ttl", fsm.Options{Registry: reg})
	ctx := context.Background()

	// Записываем с коротким TTL напрямую через Store
	_ = store.Set(ctx, "telegram", "user_ttl",
		&fsm.RawFSMState{State: string(step1)},
		150*time.Millisecond,
	)

	// Сразу — состояние есть
	current, _ := machine.Current(ctx)
	if current.State != step1 {
		t.Errorf("до истечения TTL ожидали %q, получили %q", step1, current.State)
	}

	// Ждём истечения TTL
	time.Sleep(300 * time.Millisecond)

	idle, _ := machine.IsIdle(ctx)
	if !idle {
		t.Error("после истечения TTL Current должен вернуть Idle")
	}
}

// TestIntegration_Update — Update меняет данные но не состояние.
func TestIntegration_Update(t *testing.T) {
	store := setup(t)
	machine, reg := newMachine(store, "telegram", "user_update")
	step1, _, _ := registerLinear(reg, "update_test")
	ctx := context.Background()

	initial := fsm.NewStateData()
	initial.Set("weekdays", 31) // Пн-Пт
	_ = machine.Transition(ctx, step1, initial)

	updated := fsm.NewStateData()
	updated.Set("weekdays", 63) // Пн-Сб
	if err := machine.Update(ctx, updated); err != nil {
		t.Fatalf("Update: %v", err)
	}

	current, _ := machine.Current(ctx)
	if current.State != step1 {
		t.Error("Update не должен менять состояние")
	}
	if current.Data.GetInt64("weekdays") != 63 {
		t.Errorf("weekdays = %d, ожидали 63", current.Data.GetInt64("weekdays"))
	}
}

// TestIntegration_TransitionNotAllowed — неразрешённый переход возвращает ошибку.
func TestIntegration_TransitionNotAllowed(t *testing.T) {
	store := setup(t)
	machine, reg := newMachine(store, "telegram", "user_notallowed")
	step1, _, step3 := registerLinear(reg, "notallowed")
	ctx := context.Background()

	_ = machine.Transition(ctx, step1, fsm.NewStateData())

	// Пропускаем step2 — должно вернуть ошибку
	err := machine.Transition(ctx, step3, fsm.NewStateData())
	if err != fsm.ErrTransitionNotAllowed {
		t.Errorf("ожидали ErrTransitionNotAllowed, получили %v", err)
	}

	// Состояние должно остаться step1
	current, _ := machine.Current(ctx)
	if current.State != step1 {
		t.Errorf("после неудачного перехода состояние должно остаться %q, получили %q", step1, current.State)
	}
}
