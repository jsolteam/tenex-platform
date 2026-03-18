package repositories_integration

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/jsolteam/tenex-platform/internal/domain/medicine"
	"github.com/jsolteam/tenex-platform/internal/domain/reminder"
	"github.com/jsolteam/tenex-platform/internal/domain/schedule"
	"github.com/jsolteam/tenex-platform/internal/domain/user"
	"github.com/jsolteam/tenex-platform/internal/infrastructure/database"
	"github.com/jsolteam/tenex-platform/internal/infrastructure/database/repositories"
)

func setupRepos(t *testing.T) *repositories.Repositories {
	t.Helper()
	host := os.Getenv("DB_HOST")
	if host == "" {
		t.Skip("DB_HOST not set")
	}
	cfg := database.Config{
		Host:     host,
		Port:     5432,
		User:     getEnv("DB_USER", "tenex"),
		Password: getEnv("DB_PASS", "tenex"),
		Name:     getEnv("DB_NAME", "tenex"),
		SSLMode:  getEnv("DB_SSL_MODE", "disable"),
	}
	db, err := database.OpenAndMigrate(context.Background(), cfg)
	if err != nil {
		t.Fatalf("setupRepos: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return repositories.New(db)
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func createTestUser(t *testing.T, repos *repositories.Repositories) *user.User {
	t.Helper()
	ctx := context.Background()
	u := &user.User{Timezone: "UTC", Language: "en"}
	contact := &user.UserContact{
		MessengerType:   user.MessengerTelegram,
		MessengerUserID: t.Name() + time.Now().Format("150405000000000"),
	}
	if err := repos.User.Create(ctx, u, contact); err != nil {
		t.Fatalf("createTestUser: %v", err)
	}
	return u
}

func createTestSchedule(t *testing.T, repos *repositories.Repositories, userID int64) (*medicine.Medicine, *schedule.Schedule) {
	t.Helper()
	ctx := context.Background()

	m := &medicine.Medicine{UserID: userID, Name: "TestMed"}
	if err := repos.Medicine.Create(ctx, m); err != nil {
		t.Fatalf("createTestSchedule medicine: %v", err)
	}

	today := time.Now().UTC().Truncate(24 * time.Hour)
	t1, _ := time.Parse("15:04:05", "08:00:00")
	s := &schedule.Schedule{
		MedicineID: m.ID,
		Type:       schedule.ScheduleTypeDaily,
		Times:      []time.Time{t1},
		StartDate:  today,
	}
	if err := repos.Schedule.Create(ctx, s); err != nil {
		t.Fatalf("createTestSchedule schedule: %v", err)
	}
	return m, s
}

func TestReminderRepo_CreateAndGetByID(t *testing.T) {
	repos := setupRepos(t)
	ctx := context.Background()

	u := createTestUser(t, repos)
	m, s := createTestSchedule(t, repos, u.ID)

	scheduledAt := time.Now().UTC().Truncate(time.Second)
	rem := &reminder.Reminder{
		UserID:         u.ID,
		MedicineID:     m.ID,
		ScheduleID:     s.ID,
		ScheduledAt:    scheduledAt,
		Status:         reminder.StatusPending,
		IdempotencyKey: uuid.New(),
	}

	if err := repos.Reminder.Create(ctx, rem); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if rem.ID == 0 {
		t.Fatal("expected non-zero ID after Create")
	}

	got, err := repos.Reminder.GetByID(ctx, rem.ID, scheduledAt)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.UserID != u.ID {
		t.Errorf("UserID = %d, want %d", got.UserID, u.ID)
	}
	if got.Status != reminder.StatusPending {
		t.Errorf("Status = %q, want pending", got.Status)
	}
}

func TestReminderRepo_GetByIdempotencyKey(t *testing.T) {
	repos := setupRepos(t)
	ctx := context.Background()

	u := createTestUser(t, repos)
	m, s := createTestSchedule(t, repos, u.ID)

	key := uuid.New()
	scheduledAt := time.Now().UTC().Truncate(time.Second)
	rem := &reminder.Reminder{
		UserID:         u.ID,
		MedicineID:     m.ID,
		ScheduleID:     s.ID,
		ScheduledAt:    scheduledAt,
		Status:         reminder.StatusPending,
		IdempotencyKey: key,
	}
	_ = repos.Reminder.Create(ctx, rem)

	got, err := repos.Reminder.GetByIdempotencyKey(ctx, key)
	if err != nil {
		t.Fatalf("GetByIdempotencyKey: %v", err)
	}
	if got.ID != rem.ID {
		t.Errorf("ID = %d, want %d", got.ID, rem.ID)
	}
}

func TestReminderRepo_UpdateStatus(t *testing.T) {
	repos := setupRepos(t)
	ctx := context.Background()

	u := createTestUser(t, repos)
	m, s := createTestSchedule(t, repos, u.ID)

	scheduledAt := time.Now().UTC().Truncate(time.Second)
	rem := &reminder.Reminder{
		UserID:         u.ID,
		MedicineID:     m.ID,
		ScheduleID:     s.ID,
		ScheduledAt:    scheduledAt,
		Status:         reminder.StatusPending,
		IdempotencyKey: uuid.New(),
	}
	_ = repos.Reminder.Create(ctx, rem)

	if err := repos.Reminder.UpdateStatus(ctx, rem.ID, scheduledAt, reminder.StatusSent); err != nil {
		t.Fatalf("UpdateStatus: %v", err)
	}

	got, _ := repos.Reminder.GetByID(ctx, rem.ID, scheduledAt)
	if got.Status != reminder.StatusSent {
		t.Errorf("Status = %q, want sent", got.Status)
	}
}

func TestReminderRepo_ListPending(t *testing.T) {
	repos := setupRepos(t)
	ctx := context.Background()

	u := createTestUser(t, repos)
	m, s := createTestSchedule(t, repos, u.ID)

	// Создаём 3 pending напоминания в прошлом
	now := time.Now().UTC()
	for i := 0; i < 3; i++ {
		scheduledAt := now.Add(-time.Duration(i+1) * time.Hour).Truncate(time.Second)
		rem := &reminder.Reminder{
			UserID:         u.ID,
			MedicineID:     m.ID,
			ScheduleID:     s.ID,
			ScheduledAt:    scheduledAt,
			Status:         reminder.StatusPending,
			IdempotencyKey: uuid.New(),
		}
		_ = repos.Reminder.Create(ctx, rem)
	}

	list, err := repos.Reminder.ListPending(ctx, now.Add(time.Minute), 100)
	if err != nil {
		t.Fatalf("ListPending: %v", err)
	}
	// Должно быть хотя бы 3 (могут быть и от других тестов)
	if len(list) < 3 {
		t.Errorf("ListPending returned %d, want >= 3", len(list))
	}
}

func TestReminderRepo_IncrementRetry(t *testing.T) {
	repos := setupRepos(t)
	ctx := context.Background()

	u := createTestUser(t, repos)
	m, s := createTestSchedule(t, repos, u.ID)

	scheduledAt := time.Now().UTC().Truncate(time.Second)
	rem := &reminder.Reminder{
		UserID:         u.ID,
		MedicineID:     m.ID,
		ScheduleID:     s.ID,
		ScheduledAt:    scheduledAt,
		Status:         reminder.StatusPending,
		IdempotencyKey: uuid.New(),
	}
	_ = repos.Reminder.Create(ctx, rem)

	if err := repos.Reminder.IncrementRetry(ctx, rem.ID, scheduledAt); err != nil {
		t.Fatalf("IncrementRetry: %v", err)
	}
	if err := repos.Reminder.IncrementRetry(ctx, rem.ID, scheduledAt); err != nil {
		t.Fatalf("IncrementRetry x2: %v", err)
	}

	got, _ := repos.Reminder.GetByID(ctx, rem.ID, scheduledAt)
	if got.RetryCount != 2 {
		t.Errorf("RetryCount = %d, want 2", got.RetryCount)
	}
}

func TestReminderRepo_GetByID_NotFound(t *testing.T) {
	repos := setupRepos(t)
	ctx := context.Background()

	_, err := repos.Reminder.GetByID(ctx, 999999999, time.Now())
	if err != reminder.ErrNotFound {
		t.Errorf("expected ErrNotFound, got: %v", err)
	}
}

func TestReminderRepo_ListByUser(t *testing.T) {
	repos := setupRepos(t)
	ctx := context.Background()

	u := createTestUser(t, repos)
	m, s := createTestSchedule(t, repos, u.ID)

	now := time.Now().UTC()
	for i := 0; i < 2; i++ {
		scheduledAt := now.Add(time.Duration(i) * time.Hour).Truncate(time.Second)
		rem := &reminder.Reminder{
			UserID:         u.ID,
			MedicineID:     m.ID,
			ScheduleID:     s.ID,
			ScheduledAt:    scheduledAt,
			Status:         reminder.StatusPending,
			IdempotencyKey: uuid.New(),
		}
		_ = repos.Reminder.Create(ctx, rem)
	}

	from := now.Add(-time.Hour)
	to := now.Add(2 * time.Hour)
	list, err := repos.Reminder.ListByUser(ctx, u.ID, from, to)
	if err != nil {
		t.Fatalf("ListByUser: %v", err)
	}
	if len(list) < 2 {
		t.Errorf("ListByUser returned %d, want >= 2", len(list))
	}
}
