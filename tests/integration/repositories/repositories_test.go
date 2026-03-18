package repositories_integration

import (
	"context"
	"database/sql"
	"os"
	"testing"
	"time"

	_ "github.com/lib/pq"

	"github.com/jsolteam/tenex-platform/internal/domain/medicine"
	"github.com/jsolteam/tenex-platform/internal/domain/schedule"
	"github.com/jsolteam/tenex-platform/internal/domain/user"
	"github.com/jsolteam/tenex-platform/internal/infrastructure/database"
	"github.com/jsolteam/tenex-platform/internal/infrastructure/database/repositories"
)

// Тесты запускаются только при наличии переменной DB_HOST.
// В CI используется postgres service (см. .github/workflows/ci.yml).

func setupDB(t *testing.T) (*sql.DB, *repositories.Repositories) {
	t.Helper()

	host := os.Getenv("DB_HOST")
	if host == "" {
		t.Skip("DB_HOST not set — skipping integration test")
	}

	cfg := database.Config{
		Host:     host,
		Port:     5432,
		User:     getenv("DB_USER", "tenex"),
		Password: getenv("DB_PASS", "tenex"),
		Name:     getenv("DB_NAME", "tenex"),
		SSLMode:  getenv("DB_SSL_MODE", "disable"),
	}

	db, err := database.OpenAndMigrate(context.Background(), cfg)
	if err != nil {
		t.Fatalf("setupDB: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	return db, repositories.New(db)
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// ─── User Repository ──────────────────────────────────────────────────────

func TestUserRepo_CreateAndGetByID(t *testing.T) {
	_, repos := setupDB(t)
	ctx := context.Background()

	u := &user.User{Timezone: "UTC", Language: "en"}
	contact := &user.UserContact{
		MessengerType:   user.MessengerTelegram,
		MessengerUserID: uniqueID(t),
		Username:        "testuser",
	}

	if err := repos.User.Create(ctx, u, contact); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if u.ID == 0 {
		t.Fatal("expected non-zero ID after Create")
	}

	got, err := repos.User.GetByID(ctx, u.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.Timezone != "UTC" {
		t.Errorf("Timezone = %q, want %q", got.Timezone, "UTC")
	}
}

func TestUserRepo_GetByMessenger(t *testing.T) {
	_, repos := setupDB(t)
	ctx := context.Background()

	messengerUserID := uniqueID(t)
	u := &user.User{Timezone: "Europe/Moscow", Language: "ru"}
	contact := &user.UserContact{
		MessengerType:   user.MessengerTelegram,
		MessengerUserID: messengerUserID,
	}
	if err := repos.User.Create(ctx, u, contact); err != nil {
		t.Fatalf("Create: %v", err)
	}

	got, err := repos.User.GetByMessenger(ctx, user.MessengerTelegram, messengerUserID)
	if err != nil {
		t.Fatalf("GetByMessenger: %v", err)
	}
	if got.ID != u.ID {
		t.Errorf("ID = %d, want %d", got.ID, u.ID)
	}
}

func TestUserRepo_GetByID_NotFound(t *testing.T) {
	_, repos := setupDB(t)
	ctx := context.Background()

	_, err := repos.User.GetByID(ctx, 999999999)
	if err != user.ErrNotFound {
		t.Errorf("expected ErrNotFound, got: %v", err)
	}
}

func TestUserRepo_Update(t *testing.T) {
	_, repos := setupDB(t)
	ctx := context.Background()

	u := &user.User{Timezone: "UTC", Language: "en"}
	contact := &user.UserContact{
		MessengerType:   user.MessengerTelegram,
		MessengerUserID: uniqueID(t),
	}
	if err := repos.User.Create(ctx, u, contact); err != nil {
		t.Fatalf("Create: %v", err)
	}

	u.Timezone = "Asia/Tokyo"
	u.Language = "ja"
	if err := repos.User.Update(ctx, u); err != nil {
		t.Fatalf("Update: %v", err)
	}

	got, err := repos.User.GetByID(ctx, u.ID)
	if err != nil {
		t.Fatalf("GetByID after update: %v", err)
	}
	if got.Timezone != "Asia/Tokyo" {
		t.Errorf("Timezone = %q, want Asia/Tokyo", got.Timezone)
	}
	if got.Language != "ja" {
		t.Errorf("Language = %q, want ja", got.Language)
	}
}

func TestUserRepo_AddContact_DuplicateFails(t *testing.T) {
	_, repos := setupDB(t)
	ctx := context.Background()

	messengerUserID := uniqueID(t)
	u := &user.User{Timezone: "UTC", Language: "en"}
	contact := &user.UserContact{
		MessengerType:   user.MessengerTelegram,
		MessengerUserID: messengerUserID,
	}
	if err := repos.User.Create(ctx, u, contact); err != nil {
		t.Fatalf("Create: %v", err)
	}

	// Пытаемся добавить второй контакт с тем же messenger_user_id
	dup := &user.UserContact{
		UserID:          u.ID,
		MessengerType:   user.MessengerTelegram,
		MessengerUserID: messengerUserID,
	}
	err := repos.User.AddContact(ctx, dup)
	if err != user.ErrContactAlreadyExists {
		t.Errorf("expected ErrContactAlreadyExists, got: %v", err)
	}
}

func TestUserStats_Upsert(t *testing.T) {
	_, repos := setupDB(t)
	ctx := context.Background()

	u := &user.User{Timezone: "UTC", Language: "en"}
	contact := &user.UserContact{
		MessengerType:   user.MessengerTelegram,
		MessengerUserID: uniqueID(t),
	}
	if err := repos.User.Create(ctx, u, contact); err != nil {
		t.Fatalf("Create user: %v", err)
	}

	stats := &user.UserStatistics{
		UserID:         u.ID,
		TotalReminders: 10,
		Confirmed:      8,
		Skipped:        2,
	}
	if err := repos.UserStats.Upsert(ctx, stats); err != nil {
		t.Fatalf("Upsert: %v", err)
	}

	got, err := repos.UserStats.GetByUserID(ctx, u.ID)
	if err != nil {
		t.Fatalf("GetByUserID: %v", err)
	}
	if got.Confirmed != 8 {
		t.Errorf("Confirmed = %d, want 8", got.Confirmed)
	}
	if got.AdherenceRate != 0.8 {
		t.Errorf("AdherenceRate = %v, want 0.8", got.AdherenceRate)
	}

	// Повторный upsert — обновление
	stats.Confirmed = 9
	if err := repos.UserStats.Upsert(ctx, stats); err != nil {
		t.Fatalf("Upsert 2: %v", err)
	}
	got2, _ := repos.UserStats.GetByUserID(ctx, u.ID)
	if got2.Confirmed != 9 {
		t.Errorf("after upsert Confirmed = %d, want 9", got2.Confirmed)
	}
}

// ─── Medicine Repository ──────────────────────────────────────────────────

func TestMedicineRepo_CreateListDelete(t *testing.T) {
	_, repos := setupDB(t)
	ctx := context.Background()

	// Создаём пользователя
	u := &user.User{Timezone: "UTC", Language: "en"}
	contact := &user.UserContact{
		MessengerType:   user.MessengerTelegram,
		MessengerUserID: uniqueID(t),
	}
	if err := repos.User.Create(ctx, u, contact); err != nil {
		t.Fatalf("Create user: %v", err)
	}

	// Создаём лекарство
	m := &medicine.Medicine{
		UserID: u.ID,
		Name:   "Aspirin",
	}
	if err := repos.Medicine.Create(ctx, m); err != nil {
		t.Fatalf("Create medicine: %v", err)
	}
	if m.ID == 0 {
		t.Fatal("expected non-zero medicine ID")
	}

	// Список
	list, err := repos.Medicine.ListByUser(ctx, u.ID)
	if err != nil {
		t.Fatalf("ListByUser: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("expected 1 medicine, got %d", len(list))
	}
	if list[0].Name != "Aspirin" {
		t.Errorf("Name = %q, want Aspirin", list[0].Name)
	}

	// Удаление
	if err := repos.Medicine.Delete(ctx, m.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	_, err = repos.Medicine.GetByID(ctx, m.ID)
	if err != medicine.ErrNotFound {
		t.Errorf("expected ErrNotFound after delete, got: %v", err)
	}
}

func TestMedicineRepo_Update(t *testing.T) {
	_, repos := setupDB(t)
	ctx := context.Background()

	u := &user.User{Timezone: "UTC", Language: "en"}
	contact := &user.UserContact{
		MessengerType:   user.MessengerTelegram,
		MessengerUserID: uniqueID(t),
	}
	_ = repos.User.Create(ctx, u, contact)

	m := &medicine.Medicine{UserID: u.ID, Name: "OldName"}
	_ = repos.Medicine.Create(ctx, m)

	m.Name = "NewName"
	if err := repos.Medicine.Update(ctx, m); err != nil {
		t.Fatalf("Update: %v", err)
	}

	got, err := repos.Medicine.GetByID(ctx, m.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.Name != "NewName" {
		t.Errorf("Name = %q, want NewName", got.Name)
	}
}

// ─── Schedule Repository ──────────────────────────────────────────────────

func TestScheduleRepo_CreateAndListActive(t *testing.T) {
	_, repos := setupDB(t)
	ctx := context.Background()

	// Создаём цепочку user → medicine → schedule
	u := &user.User{Timezone: "UTC", Language: "en"}
	contact := &user.UserContact{
		MessengerType:   user.MessengerTelegram,
		MessengerUserID: uniqueID(t),
	}
	_ = repos.User.Create(ctx, u, contact)

	m := &medicine.Medicine{UserID: u.ID, Name: "Vitamin D"}
	_ = repos.Medicine.Create(ctx, m)

	today := time.Now().UTC().Truncate(24 * time.Hour)
	morningTime, _ := time.Parse("15:04:05", "08:00:00")

	s := &schedule.Schedule{
		MedicineID: m.ID,
		Type:       schedule.ScheduleTypeDaily,
		Times:      []time.Time{morningTime},
		StartDate:  today,
	}
	if err := repos.Schedule.Create(ctx, s); err != nil {
		t.Fatalf("Create schedule: %v", err)
	}
	if s.ID == 0 {
		t.Fatal("expected non-zero schedule ID")
	}

	// ListActive должен вернуть расписание
	active, err := repos.Schedule.ListActive(ctx, today)
	if err != nil {
		t.Fatalf("ListActive: %v", err)
	}

	var found bool
	for _, a := range active {
		if a.ID == s.ID {
			found = true
			break
		}
	}
	if !found {
		t.Error("created schedule not found in ListActive")
	}
}

func TestScheduleRepo_Weekly(t *testing.T) {
	_, repos := setupDB(t)
	ctx := context.Background()

	u := &user.User{Timezone: "UTC", Language: "en"}
	contact := &user.UserContact{
		MessengerType:   user.MessengerTelegram,
		MessengerUserID: uniqueID(t),
	}
	_ = repos.User.Create(ctx, u, contact)

	m := &medicine.Medicine{UserID: u.ID, Name: "WeeklyMed"}
	_ = repos.Medicine.Create(ctx, m)

	today := time.Now().UTC().Truncate(24 * time.Hour)
	eveningTime, _ := time.Parse("15:04:05", "20:00:00")
	days := schedule.Weekdays

	s := &schedule.Schedule{
		MedicineID: m.ID,
		Type:       schedule.ScheduleTypeWeekly,
		DaysOfWeek: &days,
		Times:      []time.Time{eveningTime},
		StartDate:  today,
	}
	if err := repos.Schedule.Create(ctx, s); err != nil {
		t.Fatalf("Create weekly schedule: %v", err)
	}

	got, err := repos.Schedule.GetByID(ctx, s.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.DaysOfWeek == nil {
		t.Fatal("DaysOfWeek is nil after roundtrip")
	}
	if !got.DaysOfWeek.Has(schedule.Monday) {
		t.Error("Monday should be set in days_of_week")
	}
	if got.DaysOfWeek.Has(schedule.Saturday) {
		t.Error("Saturday should NOT be set in Weekdays mask")
	}
}

func TestScheduleRepo_Delete(t *testing.T) {
	_, repos := setupDB(t)
	ctx := context.Background()

	u := &user.User{Timezone: "UTC", Language: "en"}
	contact := &user.UserContact{
		MessengerType:   user.MessengerTelegram,
		MessengerUserID: uniqueID(t),
	}
	_ = repos.User.Create(ctx, u, contact)
	m := &medicine.Medicine{UserID: u.ID, Name: "DeleteMe"}
	_ = repos.Medicine.Create(ctx, m)

	today := time.Now().UTC().Truncate(24 * time.Hour)
	t1, _ := time.Parse("15:04:05", "09:00:00")
	s := &schedule.Schedule{
		MedicineID: m.ID,
		Type:       schedule.ScheduleTypeDaily,
		Times:      []time.Time{t1},
		StartDate:  today,
	}
	_ = repos.Schedule.Create(ctx, s)

	if err := repos.Schedule.Delete(ctx, s.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	_, err := repos.Schedule.GetByID(ctx, s.ID)
	if err != schedule.ErrNotFound {
		t.Errorf("expected ErrNotFound after delete, got: %v", err)
	}
}

// ─── Config Repository ────────────────────────────────────────────────────

func TestConfigRepo_GetAll(t *testing.T) {
	_, repos := setupDB(t)
	ctx := context.Background()

	// Миграция 0008 сидит дефолтные значения
	configs, err := repos.Config.GetAll(ctx)
	if err != nil {
		t.Fatalf("GetAll: %v", err)
	}
	if len(configs) == 0 {
		t.Error("expected seeded config values from migration 0008")
	}
}

func TestConfigRepo_SetAndGet(t *testing.T) {
	_, repos := setupDB(t)
	ctx := context.Background()

	key := "test.key." + uniqueID(t)
	if err := repos.Config.Set(ctx, key, "testvalue"); err != nil {
		t.Fatalf("Set: %v", err)
	}

	got, err := repos.Config.Get(ctx, key)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Value != "testvalue" {
		t.Errorf("Value = %q, want testvalue", got.Value)
	}

	// Upsert — обновление
	if err := repos.Config.Set(ctx, key, "newvalue"); err != nil {
		t.Fatalf("Set (update): %v", err)
	}
	got2, _ := repos.Config.Get(ctx, key)
	if got2.Value != "newvalue" {
		t.Errorf("after update Value = %q, want newvalue", got2.Value)
	}

	// Удаление
	if err := repos.Config.Delete(ctx, key); err != nil {
		t.Fatalf("Delete: %v", err)
	}
}

// ─── helpers ─────────────────────────────────────────────────────────────

// uniqueID генерирует уникальный строковый ID для изоляции тестов.
func uniqueID(t *testing.T) string {
	t.Helper()
	return t.Name() + "_" + time.Now().Format("150405.000000000")
}
