package redis_integration

import (
	"context"
	"testing"
	"time"

	infraredis "github.com/jsolteam/tenex-platform/internal/infrastructure/redis"
)

func TestSchedulerQueue_EnqueueAndPollDue(t *testing.T) {
	client := setup(t)
	q := infraredis.NewSchedulerQueue(client)
	ctx := context.Background()

	past := time.Now().Add(-time.Minute)
	if err := q.Enqueue(ctx, 101, past); err != nil {
		t.Fatalf("Enqueue: %v", err)
	}

	items, err := q.PollDue(ctx, 10)
	if err != nil {
		t.Fatalf("PollDue: %v", err)
	}

	var found bool
	for _, item := range items {
		if item.ReminderID == 101 {
			found = true
		}
	}
	if !found {
		t.Error("enqueued reminder not found in PollDue")
	}
}

func TestSchedulerQueue_FutureItemNotPolled(t *testing.T) {
	client := setup(t)
	q := infraredis.NewSchedulerQueue(client)
	ctx := context.Background()

	future := time.Now().Add(time.Hour)
	_ = q.Enqueue(ctx, 999, future)

	items, err := q.PollDue(ctx, 100)
	if err != nil {
		t.Fatalf("PollDue: %v", err)
	}

	for _, item := range items {
		if item.ReminderID == 999 {
			t.Error("future reminder should not appear in PollDue")
		}
	}
}

func TestSchedulerQueue_PollOrdering(t *testing.T) {
	client := setup(t)
	q := infraredis.NewSchedulerQueue(client)
	ctx := context.Background()

	now := time.Now()
	// Добавляем в обратном порядке — PollDue должен вернуть по score (ASC)
	_ = q.Enqueue(ctx, 30, now.Add(-1*time.Second))
	_ = q.Enqueue(ctx, 10, now.Add(-3*time.Second))
	_ = q.Enqueue(ctx, 20, now.Add(-2*time.Second))

	items, err := q.PollDue(ctx, 10)
	if err != nil {
		t.Fatalf("PollDue: %v", err)
	}

	// Найти позиции 10, 20, 30 в результате
	idxOf := func(id int64) int {
		for i, item := range items {
			if item.ReminderID == id {
				return i
			}
		}
		return -1
	}

	i10, i20, i30 := idxOf(10), idxOf(20), idxOf(30)
	if i10 < 0 || i20 < 0 || i30 < 0 {
		t.Fatalf("not all items found: 10=%d 20=%d 30=%d", i10, i20, i30)
	}
	if !(i10 < i20 && i20 < i30) {
		t.Errorf("wrong order: got positions 10=%d 20=%d 30=%d, want ascending", i10, i20, i30)
	}
}

func TestSchedulerQueue_Remove(t *testing.T) {
	client := setup(t)
	q := infraredis.NewSchedulerQueue(client)
	ctx := context.Background()

	_ = q.Enqueue(ctx, 42, time.Now().Add(-time.Second))

	if err := q.Remove(ctx, 42); err != nil {
		t.Fatalf("Remove: %v", err)
	}

	items, _ := q.PollDue(ctx, 100)
	for _, item := range items {
		if item.ReminderID == 42 {
			t.Error("removed reminder should not appear in PollDue")
		}
	}
}

func TestSchedulerQueue_Enqueue_Idempotent(t *testing.T) {
	client := setup(t)
	q := infraredis.NewSchedulerQueue(client)
	ctx := context.Background()

	// Один и тот же ID — дважды
	at := time.Now().Add(-time.Second)
	_ = q.Enqueue(ctx, 77, at)
	_ = q.Enqueue(ctx, 77, at)

	n, err := q.Size(ctx)
	if err != nil {
		t.Fatalf("Size: %v", err)
	}
	// Должен быть ровно 1 элемент
	if n != 1 {
		t.Errorf("Size = %d after duplicate Enqueue, want 1", n)
	}
}

func TestSchedulerQueue_EnqueueUpdatesScore(t *testing.T) {
	client := setup(t)
	q := infraredis.NewSchedulerQueue(client)
	ctx := context.Background()

	// Сначала ставим в будущее
	_ = q.Enqueue(ctx, 55, time.Now().Add(time.Hour))

	// Не должно попасть в PollDue
	items, _ := q.PollDue(ctx, 10)
	for _, item := range items {
		if item.ReminderID == 55 {
			t.Fatal("should not be due yet")
		}
	}

	// Обновляем score на прошлое
	_ = q.Enqueue(ctx, 55, time.Now().Add(-time.Second))

	items, _ = q.PollDue(ctx, 10)
	var found bool
	for _, item := range items {
		if item.ReminderID == 55 {
			found = true
		}
	}
	if !found {
		t.Error("after score update, reminder should appear in PollDue")
	}
}

func TestSchedulerQueue_PollLimit(t *testing.T) {
	client := setup(t)
	q := infraredis.NewSchedulerQueue(client)
	ctx := context.Background()

	past := time.Now().Add(-time.Second)
	for id := int64(100); id < 110; id++ {
		_ = q.Enqueue(ctx, id, past)
	}

	items, err := q.PollDue(ctx, 5)
	if err != nil {
		t.Fatalf("PollDue: %v", err)
	}
	if len(items) != 5 {
		t.Errorf("PollDue with limit=5 returned %d items, want 5", len(items))
	}
}

func TestSchedulerQueue_Size(t *testing.T) {
	client := setup(t)
	q := infraredis.NewSchedulerQueue(client)
	ctx := context.Background()

	n, err := q.Size(ctx)
	if err != nil {
		t.Fatalf("Size on empty: %v", err)
	}
	if n != 0 {
		t.Errorf("initial Size = %d, want 0", n)
	}

	_ = q.Enqueue(ctx, 1, time.Now())
	_ = q.Enqueue(ctx, 2, time.Now())

	n, _ = q.Size(ctx)
	if n != 2 {
		t.Errorf("Size after 2 enqueues = %d, want 2", n)
	}
}
