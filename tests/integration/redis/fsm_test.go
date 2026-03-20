package redis_integration

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	infraredis "github.com/jsolteam/tenex-platform/internal/infrastructure/redis"
)

const testMessenger = "telegram"

func TestFSM_SetAndGet(t *testing.T) {
	client := setup(t)
	store := infraredis.NewFSMStore(client)
	ctx := context.Background()

	userID := key(t, "user")
	state := &infraredis.FSMState{State: "add_medicine:enter_name"}

	if err := store.Set(ctx, testMessenger, userID, state, time.Minute); err != nil {
		t.Fatalf("Set: %v", err)
	}

	got, err := store.Get(ctx, testMessenger, userID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got == nil {
		t.Fatal("Get returned nil, want state")
	}
	if got.State != "add_medicine:enter_name" {
		t.Errorf("State = %q, want add_medicine:enter_name", got.State)
	}
}

func TestFSM_Get_MissReturnsNil(t *testing.T) {
	client := setup(t)
	store := infraredis.NewFSMStore(client)
	ctx := context.Background()

	got, err := store.Get(ctx, testMessenger, key(t, "ghost"))
	if err != nil {
		t.Fatalf("Get non-existent: %v", err)
	}
	if got != nil {
		t.Errorf("expected nil for missing key, got %+v", got)
	}
}

func TestFSM_SetResetsDataOnStateTransition(t *testing.T) {
	client := setup(t)
	store := infraredis.NewFSMStore(client)
	ctx := context.Background()

	userID := key(t, "user")
	payload, _ := json.Marshal(map[string]string{"name": "Aspirin"})

	// Первое состояние с данными
	_ = store.Set(ctx, testMessenger, userID, &infraredis.FSMState{
		State: "add_medicine:enter_name",
		Data:  payload,
	}, time.Minute)

	// Переход в новое состояние
	_ = store.Set(ctx, testMessenger, userID, &infraredis.FSMState{
		State: "add_medicine:enter_dose",
		Data:  payload,
	}, time.Minute)

	got, _ := store.Get(ctx, testMessenger, userID)
	if got.State != "add_medicine:enter_dose" {
		t.Errorf("State = %q, want add_medicine:enter_dose", got.State)
	}

	var data map[string]string
	_ = json.Unmarshal(got.Data, &data)
	if data["name"] != "Aspirin" {
		t.Errorf("Data.name = %q, want Aspirin", data["name"])
	}
}

func TestFSM_TTL_Expiry(t *testing.T) {
	client := setup(t)
	store := infraredis.NewFSMStore(client)
	ctx := context.Background()

	userID := key(t, "ttl")
	_ = store.Set(ctx, testMessenger, userID, &infraredis.FSMState{State: "menu"}, 100*time.Millisecond)

	time.Sleep(200 * time.Millisecond)

	got, err := store.Get(ctx, testMessenger, userID)
	if err != nil {
		t.Fatalf("Get after TTL: %v", err)
	}
	if got != nil {
		t.Error("expected nil after TTL expiry, got state")
	}
}

func TestFSM_Delete(t *testing.T) {
	client := setup(t)
	store := infraredis.NewFSMStore(client)
	ctx := context.Background()

	userID := key(t, "del")
	_ = store.Set(ctx, testMessenger, userID, &infraredis.FSMState{State: "menu"}, time.Minute)

	if err := store.Delete(ctx, testMessenger, userID); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	got, err := store.Get(ctx, testMessenger, userID)
	if err != nil {
		t.Fatalf("Get after delete: %v", err)
	}
	if got != nil {
		t.Error("expected nil after delete")
	}
}

func TestFSM_Delete_NonExistentIsNoOp(t *testing.T) {
	client := setup(t)
	store := infraredis.NewFSMStore(client)
	ctx := context.Background()

	if err := store.Delete(ctx, testMessenger, key(t, "ghost")); err != nil {
		t.Errorf("Delete non-existent should not error: %v", err)
	}
}

func TestFSM_IsolatedByMessenger(t *testing.T) {
	client := setup(t)
	store := infraredis.NewFSMStore(client)
	ctx := context.Background()

	userID := key(t, "user")
	_ = store.Set(ctx, "telegram", userID, &infraredis.FSMState{State: "tg_state"}, time.Minute)
	_ = store.Set(ctx, "vk", userID, &infraredis.FSMState{State: "vk_state"}, time.Minute)

	tg, _ := store.Get(ctx, "telegram", userID)
	vk, _ := store.Get(ctx, "vk", userID)

	if tg.State != "tg_state" {
		t.Errorf("telegram state = %q, want tg_state", tg.State)
	}
	if vk.State != "vk_state" {
		t.Errorf("vk state = %q, want vk_state", vk.State)
	}
}
