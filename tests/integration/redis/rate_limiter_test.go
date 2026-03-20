package redis_integration

import (
	"context"
	"sync"
	"testing"
	"time"

	infraredis "github.com/jsolteam/tenex-platform/internal/infrastructure/redis"
)

func TestRateLimiter_AllowsUnderLimit(t *testing.T) {
	client := setup(t)
	rl := infraredis.NewRateLimiter(client)
	ctx := context.Background()

	userID := key(t, "user")
	limit := 5
	window := time.Minute

	for i := 0; i < limit; i++ {
		allowed, err := rl.Allow(ctx, testMessenger, userID, limit, window)
		if err != nil {
			t.Fatalf("Allow[%d]: %v", i, err)
		}
		if !allowed {
			t.Errorf("request %d should be allowed (limit=%d)", i+1, limit)
		}
	}
}

func TestRateLimiter_BlocksOverLimit(t *testing.T) {
	client := setup(t)
	rl := infraredis.NewRateLimiter(client)
	ctx := context.Background()

	userID := key(t, "user")
	limit := 3

	// Исчерпываем лимит
	for i := 0; i < limit; i++ {
		_, _ = rl.Allow(ctx, testMessenger, userID, limit, time.Minute)
	}

	// Следующий запрос должен быть заблокирован
	allowed, err := rl.Allow(ctx, testMessenger, userID, limit, time.Minute)
	if err != nil {
		t.Fatalf("Allow over limit: %v", err)
	}
	if allowed {
		t.Errorf("request over limit should be blocked")
	}
}

func TestRateLimiter_WindowExpiry_ResetsCounter(t *testing.T) {
	client := setup(t)
	rl := infraredis.NewRateLimiter(client)
	ctx := context.Background()

	userID := key(t, "user")
	limit := 2
	window := 300 * time.Millisecond

	// Исчерпываем лимит
	for i := 0; i < limit; i++ {
		_, _ = rl.Allow(ctx, testMessenger, userID, limit, window)
	}

	// Убеждаемся что заблокирован
	blocked, _ := rl.Allow(ctx, testMessenger, userID, limit, window)
	if blocked {
		t.Fatal("should be blocked before window expiry")
	}

	// Ждём истечения окна (400ms > 300ms window, запас на Windows timer granularity ~15ms)
	time.Sleep(400 * time.Millisecond)

	// Теперь должен снова разрешить
	allowed, err := rl.Allow(ctx, testMessenger, userID, limit, window)
	if err != nil {
		t.Fatalf("Allow after window expiry: %v", err)
	}
	if !allowed {
		t.Error("should be allowed after sliding window expired")
	}
}

func TestRateLimiter_Reset(t *testing.T) {
	client := setup(t)
	rl := infraredis.NewRateLimiter(client)
	ctx := context.Background()

	userID := key(t, "user")
	limit := 2

	// Исчерпываем
	for i := 0; i < limit; i++ {
		_, _ = rl.Allow(ctx, testMessenger, userID, limit, time.Minute)
	}

	if err := rl.Reset(ctx, testMessenger, userID); err != nil {
		t.Fatalf("Reset: %v", err)
	}

	// После сброса должен снова разрешить
	allowed, err := rl.Allow(ctx, testMessenger, userID, limit, time.Minute)
	if err != nil {
		t.Fatalf("Allow after reset: %v", err)
	}
	if !allowed {
		t.Error("should be allowed after Reset")
	}
}

func TestRateLimiter_IsolatedByUser(t *testing.T) {
	client := setup(t)
	rl := infraredis.NewRateLimiter(client)
	ctx := context.Background()

	userA := key(t, "userA")
	userB := key(t, "userB")
	limit := 2

	// Исчерпываем A
	for i := 0; i < limit; i++ {
		_, _ = rl.Allow(ctx, testMessenger, userA, limit, time.Minute)
	}
	blockedA, _ := rl.Allow(ctx, testMessenger, userA, limit, time.Minute)
	if blockedA {
		t.Error("user A should be blocked")
	}

	// B не должен быть затронут
	allowedB, err := rl.Allow(ctx, testMessenger, userB, limit, time.Minute)
	if err != nil {
		t.Fatalf("Allow user B: %v", err)
	}
	if !allowedB {
		t.Error("user B should still be allowed")
	}
}

func TestRateLimiter_IsolatedByMessenger(t *testing.T) {
	client := setup(t)
	rl := infraredis.NewRateLimiter(client)
	ctx := context.Background()

	userID := key(t, "user")
	limit := 1

	_, _ = rl.Allow(ctx, "telegram", userID, limit, time.Minute)
	// telegram исчерпан

	blocked, _ := rl.Allow(ctx, "telegram", userID, limit, time.Minute)
	if blocked {
		t.Error("telegram should be blocked")
	}

	// vk изолирован
	allowed, err := rl.Allow(ctx, "vk", userID, limit, time.Minute)
	if err != nil {
		t.Fatalf("Allow vk: %v", err)
	}
	if !allowed {
		t.Error("vk should be independent from telegram limit")
	}
}

func TestRateLimiter_Concurrent_NoRaceCondition(t *testing.T) {
	client := setup(t)
	rl := infraredis.NewRateLimiter(client)
	ctx := context.Background()

	userID := key(t, "user")
	limit := 10
	goroutines := 20

	var wg sync.WaitGroup
	allowed := make([]bool, goroutines)
	errs := make([]error, goroutines)

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			allowed[i], errs[i] = rl.Allow(ctx, testMessenger, userID, limit, time.Minute)
		}(i)
	}
	wg.Wait()

	for i, err := range errs {
		if err != nil {
			t.Errorf("goroutine %d: %v", i, err)
		}
	}

	// Ровно limit запросов должны быть разрешены
	count := 0
	for _, a := range allowed {
		if a {
			count++
		}
	}
	if count != limit {
		t.Errorf("allowed = %d, want exactly %d under concurrent load", count, limit)
	}
}
