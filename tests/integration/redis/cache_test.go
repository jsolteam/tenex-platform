package redis_integration

import (
	"context"
	"errors"
	"testing"
	"time"

	infraredis "github.com/jsolteam/tenex-platform/internal/infrastructure/redis"
)

func TestCache_SetAndGet(t *testing.T) {
	client := setup(t)
	cache := infraredis.NewCache(client)
	ctx := context.Background()

	k := key(t, "val")
	want := []byte("hello redis")

	if err := cache.Set(ctx, k, want, time.Minute); err != nil {
		t.Fatalf("Set: %v", err)
	}

	got, err := cache.Get(ctx, k)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if string(got) != string(want) {
		t.Errorf("Get = %q, want %q", got, want)
	}
}

func TestCache_Get_MissReturnsErrCacheMiss(t *testing.T) {
	client := setup(t)
	cache := infraredis.NewCache(client)
	ctx := context.Background()

	_, err := cache.Get(ctx, key(t, "nonexistent"))
	if !errors.Is(err, infraredis.ErrCacheMiss) {
		t.Errorf("expected ErrCacheMiss, got %v", err)
	}
}

func TestCache_TTL_Expiry(t *testing.T) {
	client := setup(t)
	cache := infraredis.NewCache(client)
	ctx := context.Background()

	k := key(t, "ttl")
	if err := cache.Set(ctx, k, []byte("temp"), 100*time.Millisecond); err != nil {
		t.Fatalf("Set: %v", err)
	}

	time.Sleep(200 * time.Millisecond)

	_, err := cache.Get(ctx, k)
	if !errors.Is(err, infraredis.ErrCacheMiss) {
		t.Errorf("expected ErrCacheMiss after TTL expiry, got %v", err)
	}
}

func TestCache_Delete(t *testing.T) {
	client := setup(t)
	cache := infraredis.NewCache(client)
	ctx := context.Background()

	k := key(t, "del")
	_ = cache.Set(ctx, k, []byte("data"), time.Minute)

	if err := cache.Delete(ctx, k); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	_, err := cache.Get(ctx, k)
	if !errors.Is(err, infraredis.ErrCacheMiss) {
		t.Errorf("expected ErrCacheMiss after delete, got %v", err)
	}
}

func TestCache_Delete_NonExistentIsNoOp(t *testing.T) {
	client := setup(t)
	cache := infraredis.NewCache(client)
	ctx := context.Background()

	if err := cache.Delete(ctx, key(t, "ghost")); err != nil {
		t.Errorf("Delete non-existent key should not error: %v", err)
	}
}

func TestCache_Delete_MultipleKeys(t *testing.T) {
	client := setup(t)
	cache := infraredis.NewCache(client)
	ctx := context.Background()

	k1, k2 := key(t, "m1"), key(t, "m2")
	_ = cache.Set(ctx, k1, []byte("1"), time.Minute)
	_ = cache.Set(ctx, k2, []byte("2"), time.Minute)

	if err := cache.Delete(ctx, k1, k2); err != nil {
		t.Fatalf("Delete multiple: %v", err)
	}

	for _, k := range []string{k1, k2} {
		_, err := cache.Get(ctx, k)
		if !errors.Is(err, infraredis.ErrCacheMiss) {
			t.Errorf("key %q should be deleted, got %v", k, err)
		}
	}
}

func TestCache_Exists(t *testing.T) {
	client := setup(t)
	cache := infraredis.NewCache(client)
	ctx := context.Background()

	k := key(t, "ex")

	exists, err := cache.Exists(ctx, k)
	if err != nil {
		t.Fatalf("Exists before set: %v", err)
	}
	if exists {
		t.Error("Exists should return false before Set")
	}

	_ = cache.Set(ctx, k, []byte("v"), time.Minute)

	exists, err = cache.Exists(ctx, k)
	if err != nil {
		t.Fatalf("Exists after set: %v", err)
	}
	if !exists {
		t.Error("Exists should return true after Set")
	}
}

func TestCache_Overwrite(t *testing.T) {
	client := setup(t)
	cache := infraredis.NewCache(client)
	ctx := context.Background()

	k := key(t, "ow")
	_ = cache.Set(ctx, k, []byte("first"), time.Minute)
	_ = cache.Set(ctx, k, []byte("second"), time.Minute)

	got, _ := cache.Get(ctx, k)
	if string(got) != "second" {
		t.Errorf("Get after overwrite = %q, want second", got)
	}
}
