package redis_integration

import (
	"context"
	"errors"
	"testing"
	"time"

	infraredis "github.com/jsolteam/tenex-platform/internal/infrastructure/redis"
)

func TestLock_AcquireAndRelease(t *testing.T) {
	client := setup(t)
	locker := infraredis.NewDistributedLocker(client)
	ctx := context.Background()

	id := key(t, "res")
	lock, err := locker.Acquire(ctx, "reminder", id, time.Minute)
	if err != nil {
		t.Fatalf("Acquire: %v", err)
	}
	if lock == nil {
		t.Fatal("expected non-nil lock")
	}

	if err := lock.Release(ctx); err != nil {
		t.Fatalf("Release: %v", err)
	}
}

func TestLock_DoubleAcquire_SecondFails(t *testing.T) {
	client := setup(t)
	locker := infraredis.NewDistributedLocker(client)
	ctx := context.Background()

	id := key(t, "res")
	lock1, err := locker.Acquire(ctx, "reminder", id, time.Minute)
	if err != nil {
		t.Fatalf("first Acquire: %v", err)
	}
	defer lock1.Release(ctx)

	_, err = locker.Acquire(ctx, "reminder", id, time.Minute)
	if !errors.Is(err, infraredis.ErrLockNotAcquired) {
		t.Errorf("second Acquire should return ErrLockNotAcquired, got %v", err)
	}
}

func TestLock_AcquireAfterRelease(t *testing.T) {
	client := setup(t)
	locker := infraredis.NewDistributedLocker(client)
	ctx := context.Background()

	id := key(t, "res")
	lock, _ := locker.Acquire(ctx, "reminder", id, time.Minute)
	_ = lock.Release(ctx)

	// После Release другой воркер должен захватить лок
	lock2, err := locker.Acquire(ctx, "reminder", id, time.Minute)
	if err != nil {
		t.Fatalf("Acquire after Release: %v", err)
	}
	defer lock2.Release(ctx)
}

func TestLock_TTL_AutoRelease(t *testing.T) {
	client := setup(t)
	locker := infraredis.NewDistributedLocker(client)
	ctx := context.Background()

	id := key(t, "res")
	// Короткий TTL — имитирует упавший воркер
	_, err := locker.Acquire(ctx, "reminder", id, 150*time.Millisecond)
	if err != nil {
		t.Fatalf("Acquire: %v", err)
	}
	// НЕ вызываем Release — должен истечь сам

	time.Sleep(250 * time.Millisecond)

	// Теперь другой воркер должен захватить
	lock2, err := locker.Acquire(ctx, "reminder", id, time.Minute)
	if err != nil {
		t.Fatalf("Acquire after TTL: %v", err)
	}
	defer lock2.Release(ctx)
}

func TestLock_Release_IdempotentAfterTTL(t *testing.T) {
	client := setup(t)
	locker := infraredis.NewDistributedLocker(client)
	ctx := context.Background()

	id := key(t, "res")
	lock, _ := locker.Acquire(ctx, "reminder", id, 100*time.Millisecond)

	time.Sleep(200 * time.Millisecond)

	// Лок уже истёк — Release не должен вернуть ошибку
	if err := lock.Release(ctx); err != nil {
		t.Errorf("Release on expired lock should not error: %v", err)
	}
}

func TestLock_TokenSafety_OtherReleaseFails(t *testing.T) {
	client := setup(t)
	locker := infraredis.NewDistributedLocker(client)
	ctx := context.Background()

	id := key(t, "res")
	// Первый воркер захватывает
	lock1, _ := locker.Acquire(ctx, "reminder", id, time.Minute)

	// Второй воркер не смог захватить, но пытается удалить чужой лок через второй lock-объект
	// (симулируем ситуацию где воркер имеет устаревшую ссылку)
	lock1.Release(ctx) // освобождаем
	lock2, _ := locker.Acquire(ctx, "reminder", id, time.Minute)
	defer lock2.Release(ctx)

	// Повторный Release от lock1 (старый токен) не должен удалить lock2
	_ = lock1.Release(ctx)

	// lock2 должен всё ещё удерживать лок —
	// третий Acquire должен провалиться
	_, err := locker.Acquire(ctx, "reminder", id, time.Minute)
	if !errors.Is(err, infraredis.ErrLockNotAcquired) {
		t.Errorf("lock2 should still be held after stale lock1.Release, got %v", err)
	}
}

func TestLock_IsolatedByNamespace(t *testing.T) {
	client := setup(t)
	locker := infraredis.NewDistributedLocker(client)
	ctx := context.Background()

	id := key(t, "same-id")

	lock1, err := locker.Acquire(ctx, "reminder", id, time.Minute)
	if err != nil {
		t.Fatalf("Acquire namespace=reminder: %v", err)
	}
	defer lock1.Release(ctx)

	// Тот же ID, другое пространство имён — должен захватить
	lock2, err := locker.Acquire(ctx, "worker", id, time.Minute)
	if err != nil {
		t.Fatalf("Acquire namespace=worker: %v", err)
	}
	defer lock2.Release(ctx)
}
