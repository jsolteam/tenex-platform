package redis

import (
	"context"
	"fmt"
	"strconv"
	"time"

	goredis "github.com/redis/go-redis/v9"
	"go.opentelemetry.io/otel/attribute"
	"go.uber.org/zap"

	"github.com/jsolteam/tenex-platform/internal/infrastructure/infralog"
	apperrors "github.com/jsolteam/tenex-platform/internal/platform/errors"
	"github.com/jsolteam/tenex-platform/internal/platform/observability/metrics"
)

const schedulerQueueKey = "scheduler:reminders"

// ScheduledItem — элемент очереди: ID напоминания и запланированное время.
type ScheduledItem struct {
	ReminderID  int64
	ScheduledAt time.Time
}

// SchedulerQueue — Redis ZSET очередь напоминаний.
//
// Структура:
//
//	Key:    scheduler:reminders
//	Member: "{reminderID}" (строка)
//	Score:  Unix timestamp запланированного времени
//
// Воркер делает ZRANGEBYSCORE 0 now LIMIT N, обрабатывает пачку,
// после успешной обработки вызывает Remove.
type SchedulerQueue struct {
	client *Client
	met    *metrics.RedisMetrics
}

// NewSchedulerQueue создаёт SchedulerQueue поверх существующего Client.
func NewSchedulerQueue(c *Client) *SchedulerQueue {
	return &SchedulerQueue{client: c, met: c.met}
}

// Enqueue добавляет напоминание в очередь.
// Если элемент уже существует — обновляет score (idempotent).
func (q *SchedulerQueue) Enqueue(ctx context.Context, reminderID int64, at time.Time) error {
	ctx, span := q.client.tracer.Start(ctx, "scheduler_queue.Enqueue")
	defer span.End()
	start := time.Now()
	defer func() {
		q.met.RecordDuration(ctx, metrics.RedisComponentQueue, "Enqueue", time.Since(start).Seconds())
	}()
	span.SetAttributes(
		attribute.Int64("reminder.id", reminderID),
		attribute.String("reminder.scheduled_at", at.UTC().Format(time.RFC3339)),
	)

	member := memberKey(reminderID)
	score := float64(at.UTC().Unix())

	if err := q.client.rdb.ZAdd(ctx, schedulerQueueKey, goredis.Z{
		Score:  score,
		Member: member,
	}).Err(); err != nil {
		appErr := apperrors.Redis("scheduler_queue.Enqueue", err).
			With("reminder_id", reminderID)
		infralog.Err(ctx, q.client.log, span, q.met,
			metrics.RedisComponentQueue, "Enqueue", "scheduler enqueue failed", appErr,
			zap.Int64("reminder_id", reminderID),
		)
		return appErr
	}

	infralog.Debug(ctx, q.client.log, "reminder enqueued",
		zap.Int64("reminder_id", reminderID),
		zap.Time("scheduled_at", at),
	)
	return nil
}

// PollDue возвращает до limit напоминаний, время которых наступило (score <= now).
// Не удаляет элементы из очереди — это обязанность воркера после обработки.
func (q *SchedulerQueue) PollDue(ctx context.Context, limit int) ([]ScheduledItem, error) {
	ctx, span := q.client.tracer.Start(ctx, "scheduler_queue.PollDue")
	defer span.End()
	start := time.Now()
	defer func() {
		q.met.RecordDuration(ctx, metrics.RedisComponentQueue, "PollDue", time.Since(start).Seconds())
	}()

	now := float64(time.Now().UTC().Unix())

	results, err := q.client.rdb.ZRangeByScoreWithScores(ctx, schedulerQueueKey, &goredis.ZRangeBy{
		Min:    "0",
		Max:    strconv.FormatFloat(now, 'f', 0, 64),
		Offset: 0,
		Count:  int64(limit),
	}).Result()
	if err != nil {
		appErr := apperrors.Redis("scheduler_queue.PollDue", err)
		infralog.Err(ctx, q.client.log, span, q.met,
			metrics.RedisComponentQueue, "PollDue", "scheduler poll failed", appErr)
		return nil, appErr
	}

	items := make([]ScheduledItem, 0, len(results))
	for _, z := range results {
		id, err := parseMemberKey(z.Member.(string))
		if err != nil {
			infralog.Debug(ctx, q.client.log, "scheduler: invalid member key",
				zap.String("member", z.Member.(string)),
				zap.Error(err),
			)
			continue
		}
		items = append(items, ScheduledItem{
			ReminderID:  id,
			ScheduledAt: time.Unix(int64(z.Score), 0).UTC(),
		})
	}

	span.SetAttributes(attribute.Int("scheduler.polled", len(items)))
	return items, nil
}

// Remove удаляет напоминание из очереди после успешной обработки.
func (q *SchedulerQueue) Remove(ctx context.Context, reminderID int64) error {
	ctx, span := q.client.tracer.Start(ctx, "scheduler_queue.Remove")
	defer span.End()
	start := time.Now()
	defer func() {
		q.met.RecordDuration(ctx, metrics.RedisComponentQueue, "Remove", time.Since(start).Seconds())
	}()
	span.SetAttributes(attribute.Int64("reminder.id", reminderID))

	if err := q.client.rdb.ZRem(ctx, schedulerQueueKey, memberKey(reminderID)).Err(); err != nil {
		appErr := apperrors.Redis("scheduler_queue.Remove", err).
			With("reminder_id", reminderID)
		infralog.Err(ctx, q.client.log, span, q.met,
			metrics.RedisComponentQueue, "Remove", "scheduler remove failed", appErr,
			zap.Int64("reminder_id", reminderID),
		)
		return appErr
	}
	return nil
}

// Size возвращает текущее количество элементов в очереди.
// Используется в метриках и health-checks.
func (q *SchedulerQueue) Size(ctx context.Context) (int64, error) {
	ctx, span := q.client.tracer.Start(ctx, "scheduler_queue.Size")
	defer span.End()
	start := time.Now()
	defer func() {
		q.met.RecordDuration(ctx, metrics.RedisComponentQueue, "Size", time.Since(start).Seconds())
	}()

	n, err := q.client.rdb.ZCard(ctx, schedulerQueueKey).Result()
	if err != nil {
		appErr := apperrors.Redis("scheduler_queue.Size", err)
		infralog.Err(ctx, q.client.log, span, q.met,
			metrics.RedisComponentQueue, "Size", "scheduler size failed", appErr)
		return 0, appErr
	}
	return n, nil
}

func memberKey(reminderID int64) string {
	return fmt.Sprintf("%d", reminderID)
}

func parseMemberKey(s string) (int64, error) {
	return strconv.ParseInt(s, 10, 64)
}
