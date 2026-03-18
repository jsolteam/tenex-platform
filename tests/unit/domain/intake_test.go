package domain_unit

import (
	"testing"

	"github.com/jsolteam/tenex-platform/internal/domain/intake"
)

func TestIntakeStatus_Values(t *testing.T) {
	statuses := []intake.Status{
		intake.StatusConfirmed,
		intake.StatusSkipped,
	}
	seen := make(map[intake.Status]bool)
	for _, s := range statuses {
		if seen[s] {
			t.Errorf("duplicate Status: %q", s)
		}
		seen[s] = true
		if s == "" {
			t.Error("Status must not be empty")
		}
	}
}

func TestIntake_SkippedRequiresReason(t *testing.T) {
	// Доменное правило: skipped intake должен иметь reason.
	// Проверяем что ErrReasonRequired существует и явный.
	if intake.ErrReasonRequired == nil {
		t.Error("ErrReasonRequired must not be nil")
	}
}

func TestIntake_ProofOptional(t *testing.T) {
	// ProofMediaID может быть nil (не все пользователи прикрепляют фото)
	i := intake.Intake{
		ReminderID: 1,
		Status:     intake.StatusConfirmed,
		// ProofMediaID — nil
	}
	if i.ProofMediaID != nil {
		t.Error("ProofMediaID should default to nil")
	}
}
