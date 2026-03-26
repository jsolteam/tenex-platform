package fsm_unit

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/jsolteam/tenex-platform/internal/fsm"
)

// ── StateData ─────────────────────────────────────────────────────────────────

func TestStateData_SetGet(t *testing.T) {
	d := fsm.NewStateData()
	d.Set("name", "Аспирин")
	d.Set("count", int64(3))
	d.Set("active", true)

	if got := d.GetString("name"); got != "Аспирин" {
		t.Errorf("GetString = %q, ожидали Аспирин", got)
	}
	if got := d.GetInt64("count"); got != 3 {
		t.Errorf("GetInt64 = %d, ожидали 3", got)
	}
	if got := d.GetBool("active"); !got {
		t.Error("GetBool = false, ожидали true")
	}
}

func TestStateData_GetMissingReturnsZero(t *testing.T) {
	d := fsm.NewStateData()
	if d.GetString("missing") != "" {
		t.Error("GetString несуществующего ключа должна вернуть пустую строку")
	}
	if d.GetInt64("missing") != 0 {
		t.Error("GetInt64 несуществующего ключа должна вернуть 0")
	}
	if d.GetBool("missing") {
		t.Error("GetBool несуществующего ключа должна вернуть false")
	}
	if d.GetStrings("missing") != nil {
		t.Error("GetStrings несуществующего ключа должна вернуть nil")
	}
}

func TestStateData_Has(t *testing.T) {
	d := fsm.NewStateData()
	if d.Has("key") {
		t.Error("Has должен вернуть false для отсутствующего ключа")
	}
	d.Set("key", nil)
	if !d.Has("key") {
		t.Error("Has должен вернуть true даже для nil-значения")
	}
}

func TestStateData_Delete(t *testing.T) {
	d := fsm.NewStateData()
	d.Set("key", "value")
	d.Delete("key")
	if d.Has("key") {
		t.Error("ключ должен быть удалён")
	}
}

func TestStateData_Clone(t *testing.T) {
	original := fsm.NewStateData()
	original.Set("key", "original")

	clone := original.Clone()
	clone.Set("key", "changed")

	// Изменение клона не должно влиять на оригинал
	if original.GetString("key") != "original" {
		t.Error("Clone должен возвращать независимую копию")
	}
}

func TestStateData_GetInt64_FromFloat64(t *testing.T) {
	// json.Unmarshal декодирует числа как float64 — GetInt64 должен обрабатывать это
	d := fsm.NewStateData()
	raw := `{"reminder_id": 42}`
	if err := d.UnmarshalJSON([]byte(raw)); err != nil {
		t.Fatalf("UnmarshalJSON: %v", err)
	}
	if got := d.GetInt64("reminder_id"); got != 42 {
		t.Errorf("GetInt64 после JSON roundtrip = %d, ожидали 42", got)
	}
}

func TestStateData_GetStrings_FromJSONArray(t *testing.T) {
	d := fsm.NewStateData()
	raw := `{"times": ["08:00", "20:00"]}`
	if err := d.UnmarshalJSON([]byte(raw)); err != nil {
		t.Fatalf("UnmarshalJSON: %v", err)
	}
	got := d.GetStrings("times")
	if len(got) != 2 || got[0] != "08:00" || got[1] != "20:00" {
		t.Errorf("GetStrings = %v, ожидали [08:00 20:00]", got)
	}
}

func TestStateData_JSONRoundtrip(t *testing.T) {
	original := fsm.NewStateData()
	original.Set("name", "Ибупрофен")
	original.Set("times", []interface{}{"08:00", "20:00"})

	b, err := original.MarshalJSON()
	if err != nil {
		t.Fatalf("MarshalJSON: %v", err)
	}

	restored := fsm.NewStateData()
	if err := restored.UnmarshalJSON(b); err != nil {
		t.Fatalf("UnmarshalJSON: %v", err)
	}

	if restored.GetString("name") != "Ибупрофен" {
		t.Errorf("name после roundtrip = %q", restored.GetString("name"))
	}
}

// ── State ──────────────────────────────────────────────────────────────────────

func TestNewState_Format(t *testing.T) {
	s := fsm.NewState("add_medicine", "name")
	if string(s) != "add_medicine:name" {
		t.Errorf("NewState = %q, ожидали add_medicine:name", s)
	}
}

func TestNewState_PanicsOnEmpty(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("NewState с пустым scenario должен паниковать")
		}
	}()
	fsm.NewState("", "step")
}

func TestState_IsIdle(t *testing.T) {
	if !fsm.Idle.IsIdle() {
		t.Error("Idle.IsIdle() должен возвращать true")
	}
	s := fsm.NewState("add_medicine", "name")
	if s.IsIdle() {
		t.Error("не-Idle состояние не должно возвращать IsIdle() = true")
	}
}

func TestState_String(t *testing.T) {
	if fsm.Idle.String() != "(idle)" {
		t.Errorf("Idle.String() = %q, ожидали (idle)", fsm.Idle.String())
	}
	s := fsm.NewState("reminder", "skip_reason")
	if s.String() != "reminder:skip_reason" {
		t.Errorf("State.String() = %q", s.String())
	}
}

// ── Registry ───────────────────────────────────────────────────────────────────

func TestRegistry_IsAllowed_IdleAlwaysPermitted(t *testing.T) {
	reg := fsm.NewRegistry()
	if !reg.IsAllowed(fsm.NewState("any", "state"), fsm.Idle) {
		t.Error("переход в Idle должен быть разрешён из любого состояния")
	}
}

func TestRegistry_DotNotEmpty(t *testing.T) {
	// GlobalRegistry содержит все сценарии из scenarios/
	// Проверяем что Dot генерирует непустой граф
	reg := fsm.GlobalRegistry()
	dot := reg.Dot()
	if len(dot) < 10 {
		t.Error("Dot() вернул пустой или слишком короткий граф")
	}
	if dot[:len("digraph")] != "digraph" {
		t.Error("Dot() должен начинаться с 'digraph'")
	}
}

// ── FSMState ───────────────────────────────────────────────────────────────────

func TestFSMState_NewIdleFSMState(t *testing.T) {
	s := fsm.NewIdleFSMState()
	if !s.State.IsIdle() {
		t.Error("NewIdleFSMState должен создавать состояние Idle")
	}
	if s.Data.Has("anything") {
		t.Error("NewIdleFSMState должен создавать пустой StateData")
	}
}

func TestFSMState_JSONRoundtrip(t *testing.T) {
	original := fsm.NewFSMState(fsm.NewState("add_medicine", "name"))
	original.Data.Set("name", "Парацетамол")

	b, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	var restored fsm.FSMState
	if err := json.Unmarshal(b, &restored); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if restored.State != original.State {
		t.Errorf("State = %q, ожидали %q", restored.State, original.State)
	}
	if restored.Data.GetString("name") != "Парацетамол" {
		t.Errorf("Data.name = %q после roundtrip", restored.Data.GetString("name"))
	}
}

// ── Machine с mock-store ───────────────────────────────────────────────────────

// mockFSMStore имитирует redis.FSMStore в памяти.
type mockFSMStore struct {
	data map[string]string // key -> JSON
}

func newMockFSMStore() *mockFSMStore {
	return &mockFSMStore{data: make(map[string]string)}
}

// Заглушки методов — Machine обращается к store через внутренние методы,
// поэтому тестируем Machine через публичный интерфейс.
// Реальное тестирование redisMachine — в integration тестах.

func TestMachine_CurrentReturnsIdleWhenNoState(t *testing.T) {
	// Создаём Registry изолированно, без глобального состояния
	reg := newIsolatedRegistry()
	store := newInMemoryStore()
	machine := fsm.NewWithStore(store, "telegram", "user1", fsm.Options{
		Registry: reg,
	})

	current, err := machine.Current(context.Background())
	if err != nil {
		t.Fatalf("Current: %v", err)
	}
	if !current.State.IsIdle() {
		t.Errorf("Current без состояния должен вернуть Idle, получили %q", current.State)
	}
}

func TestMachine_TransitionAndCurrent(t *testing.T) {
	reg := newIsolatedRegistry()
	stateA := fsm.NewState("test_scenario", "step_a")
	stateB := fsm.NewState("test_scenario", "step_b")

	reg.RegisterState(stateA, "Шаг A")
	reg.RegisterState(stateB, "Шаг B")
	reg.MustRegister(
		fsm.TransitionRule{From: fsm.Idle, To: stateA, Description: "начало"},
		fsm.TransitionRule{From: stateA, To: stateB, Description: "a→b"},
	)

	store := newInMemoryStore()
	machine := fsm.NewWithStore(store, "telegram", "user1", fsm.Options{Registry: reg})
	ctx := context.Background()

	data := fsm.NewStateData()
	data.Set("field", "value")

	if err := machine.Transition(ctx, stateA, data); err != nil {
		t.Fatalf("Transition → stateA: %v", err)
	}

	current, err := machine.Current(ctx)
	if err != nil {
		t.Fatalf("Current: %v", err)
	}
	if current.State != stateA {
		t.Errorf("Current.State = %q, ожидали %q", current.State, stateA)
	}
	if current.Data.GetString("field") != "value" {
		t.Errorf("данные не сохранились в состоянии")
	}
}

func TestMachine_TransitionNotAllowed(t *testing.T) {
	reg := newIsolatedRegistry()
	stateA := fsm.NewState("test_notallowed", "a")
	stateB := fsm.NewState("test_notallowed", "b")
	reg.RegisterState(stateA, "A")
	reg.RegisterState(stateB, "B")
	// Переход A→B не регистрируем — только Idle→A

	reg.MustRegister(fsm.TransitionRule{From: fsm.Idle, To: stateA, Description: "старт"})

	store := newInMemoryStore()
	machine := fsm.NewWithStore(store, "telegram", "user1", fsm.Options{Registry: reg})
	ctx := context.Background()

	_ = machine.Transition(ctx, stateA, fsm.NewStateData())

	err := machine.Transition(ctx, stateB, fsm.NewStateData())
	if err != fsm.ErrTransitionNotAllowed {
		t.Errorf("ожидали ErrTransitionNotAllowed, получили %v", err)
	}
}

func TestMachine_Reset(t *testing.T) {
	reg := newIsolatedRegistry()
	stateA := fsm.NewState("test_reset", "a")
	reg.RegisterState(stateA, "A")
	reg.MustRegister(fsm.TransitionRule{From: fsm.Idle, To: stateA, Description: "старт"})

	store := newInMemoryStore()
	machine := fsm.NewWithStore(store, "telegram", "user1", fsm.Options{Registry: reg})
	ctx := context.Background()

	_ = machine.Transition(ctx, stateA, fsm.NewStateData())

	if err := machine.Reset(ctx); err != nil {
		t.Fatalf("Reset: %v", err)
	}

	idle, err := machine.IsIdle(ctx)
	if err != nil {
		t.Fatalf("IsIdle: %v", err)
	}
	if !idle {
		t.Error("после Reset пользователь должен быть в состоянии Idle")
	}
}

func TestMachine_Update(t *testing.T) {
	reg := newIsolatedRegistry()
	stateA := fsm.NewState("test_update", "a")
	reg.RegisterState(stateA, "A")
	reg.MustRegister(fsm.TransitionRule{From: fsm.Idle, To: stateA, Description: "старт"})

	store := newInMemoryStore()
	machine := fsm.NewWithStore(store, "telegram", "user1", fsm.Options{Registry: reg})
	ctx := context.Background()

	initial := fsm.NewStateData()
	initial.Set("step", "1")
	_ = machine.Transition(ctx, stateA, initial)

	updated := fsm.NewStateData()
	updated.Set("step", "2")
	updated.Set("extra", "данные")
	if err := machine.Update(ctx, updated); err != nil {
		t.Fatalf("Update: %v", err)
	}

	current, _ := machine.Current(ctx)
	if current.State != stateA {
		t.Error("Update не должен менять состояние")
	}
	if current.Data.GetString("step") != "2" {
		t.Error("Update должен обновить данные")
	}
}

func TestMachine_InState(t *testing.T) {
	reg := newIsolatedRegistry()
	stateA := fsm.NewState("test_instate", "a")
	stateB := fsm.NewState("test_instate", "b")
	reg.RegisterState(stateA, "A")
	reg.RegisterState(stateB, "B")
	reg.MustRegister(fsm.TransitionRule{From: fsm.Idle, To: stateA, Description: "старт"})

	store := newInMemoryStore()
	machine := fsm.NewWithStore(store, "telegram", "user1", fsm.Options{Registry: reg})
	ctx := context.Background()

	_ = machine.Transition(ctx, stateA, fsm.NewStateData())

	inA, _ := machine.InState(ctx, stateA)
	inB, _ := machine.InState(ctx, stateB)

	if !inA {
		t.Error("InState(stateA) должен вернуть true")
	}
	if inB {
		t.Error("InState(stateB) должен вернуть false")
	}
}

func TestMachine_IsolationByUserID(t *testing.T) {
	reg := newIsolatedRegistry()
	stateA := fsm.NewState("test_isolation", "a")
	reg.RegisterState(stateA, "A")
	reg.MustRegister(fsm.TransitionRule{From: fsm.Idle, To: stateA, Description: "старт"})

	store := newInMemoryStore()
	machine1 := fsm.NewWithStore(store, "telegram", "user1", fsm.Options{Registry: reg})
	machine2 := fsm.NewWithStore(store, "telegram", "user2", fsm.Options{Registry: reg})
	ctx := context.Background()

	_ = machine1.Transition(ctx, stateA, fsm.NewStateData())

	idle2, _ := machine2.IsIdle(ctx)
	if !idle2 {
		t.Error("состояние user1 не должно влиять на user2")
	}
}

func TestMachine_IsolationByMessenger(t *testing.T) {
	reg := newIsolatedRegistry()
	stateA := fsm.NewState("test_messenger_iso", "a")
	reg.RegisterState(stateA, "A")
	reg.MustRegister(fsm.TransitionRule{From: fsm.Idle, To: stateA, Description: "старт"})

	store := newInMemoryStore()
	machTG := fsm.NewWithStore(store, "telegram", "user1", fsm.Options{Registry: reg})
	machVK := fsm.NewWithStore(store, "vk", "user1", fsm.Options{Registry: reg})
	ctx := context.Background()

	_ = machTG.Transition(ctx, stateA, fsm.NewStateData())

	idleVK, _ := machVK.IsIdle(ctx)
	if !idleVK {
		t.Error("состояние telegram не должно влиять на vk для того же userID")
	}
}

func TestMachine_TTLExpiry(t *testing.T) {
	// Используем store с коротким TTL для проверки истечения
	reg := newIsolatedRegistry()
	stateA := fsm.NewState("test_ttl", "a")
	reg.RegisterState(stateA, "A")
	reg.MustRegister(fsm.TransitionRule{From: fsm.Idle, To: stateA, Description: "старт"})

	store := newInMemoryStoreWithTTL(50 * time.Millisecond)
	machine := fsm.NewWithStore(store, "telegram", "user1", fsm.Options{Registry: reg})
	ctx := context.Background()

	_ = machine.Transition(ctx, stateA, fsm.NewStateData())

	// Ждём истечения TTL
	time.Sleep(100 * time.Millisecond)

	idle, _ := machine.IsIdle(ctx)
	if !idle {
		t.Error("после истечения TTL Current должен вернуть Idle")
	}
}

// ── вспомогательные типы для unit-тестов ──────────────────────────────────────

// newIsolatedRegistry создаёт изолированный реестр без глобального состояния.
func newIsolatedRegistry() *fsm.Registry {
	return fsm.NewRegistry()
}
