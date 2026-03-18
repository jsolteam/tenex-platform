package domain_unit

import (
	"testing"

	"github.com/jsolteam/tenex-platform/internal/domain/config"
	"github.com/jsolteam/tenex-platform/internal/domain/intake"
	"github.com/jsolteam/tenex-platform/internal/domain/media"
	"github.com/jsolteam/tenex-platform/internal/domain/medicine"
	"github.com/jsolteam/tenex-platform/internal/domain/reminder"
	"github.com/jsolteam/tenex-platform/internal/domain/schedule"
	"github.com/jsolteam/tenex-platform/internal/domain/user"
	"github.com/jsolteam/tenex-platform/internal/domain/watcher"
)

func TestDomainErrors_NotNil(t *testing.T) {
	errs := map[string]error{
		// user
		"user.ErrNotFound":             user.ErrNotFound,
		"user.ErrContactNotFound":      user.ErrContactNotFound,
		"user.ErrContactAlreadyExists": user.ErrContactAlreadyExists,
		"user.ErrInvalidTimezone":      user.ErrInvalidTimezone,
		"user.ErrInvalidLanguage":      user.ErrInvalidLanguage,
		// media
		"media.ErrNotFound":           media.ErrNotFound,
		"media.ErrVariantNotFound":    media.ErrVariantNotFound,
		"media.ErrInvalidMediaType":   media.ErrInvalidMediaType,
		"media.ErrInvalidStorageType": media.ErrInvalidStorageType,
		// medicine
		"medicine.ErrNotFound":    medicine.ErrNotFound,
		"medicine.ErrEmptyName":   medicine.ErrEmptyName,
		"medicine.ErrNameTooLong": medicine.ErrNameTooLong,
		// schedule
		"schedule.ErrNotFound":           schedule.ErrNotFound,
		"schedule.ErrEmptyTimes":         schedule.ErrEmptyTimes,
		"schedule.ErrInvalidType":        schedule.ErrInvalidType,
		"schedule.ErrIntervalRequired":   schedule.ErrIntervalRequired,
		"schedule.ErrDaysOfWeekRequired": schedule.ErrDaysOfWeekRequired,
		"schedule.ErrInvalidDateRange":   schedule.ErrInvalidDateRange,
		// reminder
		"reminder.ErrNotFound":          reminder.ErrNotFound,
		"reminder.ErrAlreadyExists":     reminder.ErrAlreadyExists,
		"reminder.ErrInvalidStatus":     reminder.ErrInvalidStatus,
		"reminder.ErrFinalStatus":       reminder.ErrFinalStatus,
		"reminder.ErrMaxRetriesReached": reminder.ErrMaxRetriesReached,
		// intake
		"intake.ErrNotFound":       intake.ErrNotFound,
		"intake.ErrAlreadyExists":  intake.ErrAlreadyExists,
		"intake.ErrReasonRequired": intake.ErrReasonRequired,
		"intake.ErrInvalidStatus":  intake.ErrInvalidStatus,
		// watcher
		"watcher.ErrNotFound":                watcher.ErrNotFound,
		"watcher.ErrAlreadyExists":           watcher.ErrAlreadyExists,
		"watcher.ErrSelfWatch":               watcher.ErrSelfWatch,
		"watcher.ErrNotificationNotFound":    watcher.ErrNotificationNotFound,
		"watcher.ErrNotificationAlreadySent": watcher.ErrNotificationAlreadySent,
		"watcher.ErrInvalidEvent":            watcher.ErrInvalidEvent,
		// config
		"config.ErrNotFound":   config.ErrNotFound,
		"config.ErrEmptyKey":   config.ErrEmptyKey,
		"config.ErrEmptyValue": config.ErrEmptyValue,
	}

	for name, err := range errs {
		if err == nil {
			t.Errorf("%s is nil", name)
		}
	}
}

func TestDomainErrors_Unique(t *testing.T) {
	// Ошибки внутри одного пакета должны быть уникальны
	userErrors := []error{
		user.ErrNotFound,
		user.ErrContactNotFound,
		user.ErrContactAlreadyExists,
		user.ErrInvalidTimezone,
		user.ErrInvalidLanguage,
	}
	assertUnique(t, "user", userErrors)

	reminderErrors := []error{
		reminder.ErrNotFound,
		reminder.ErrAlreadyExists,
		reminder.ErrInvalidStatus,
		reminder.ErrFinalStatus,
		reminder.ErrMaxRetriesReached,
	}
	assertUnique(t, "reminder", reminderErrors)

	watcherErrors := []error{
		watcher.ErrNotFound,
		watcher.ErrAlreadyExists,
		watcher.ErrSelfWatch,
		watcher.ErrNotificationNotFound,
		watcher.ErrNotificationAlreadySent,
		watcher.ErrInvalidEvent,
	}
	assertUnique(t, "watcher", watcherErrors)
}

func assertUnique(t *testing.T, pkg string, errs []error) {
	t.Helper()
	seen := make(map[string]bool)
	for _, err := range errs {
		msg := err.Error()
		if seen[msg] {
			t.Errorf("package %q has duplicate error message: %q", pkg, msg)
		}
		seen[msg] = true
	}
}
