package domain_unit

import (
	"testing"

	"github.com/jsolteam/tenex-platform/internal/domain/user"
)

func TestUserStatistics_AdherenceRate(t *testing.T) {
	tests := []struct {
		name          string
		total         int
		confirmed     int
		wantAdherence float64
	}{
		{"perfect adherence", 10, 10, 1.0},
		{"half adherence", 10, 5, 0.5},
		{"zero adherence", 10, 0, 0.0},
		{"zero total", 0, 0, 0.0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := user.UserStatistics{
				TotalReminders: tt.total,
				Confirmed:      tt.confirmed,
			}
			var rate float64
			if s.TotalReminders > 0 {
				rate = float64(s.Confirmed) / float64(s.TotalReminders)
			}
			if rate != tt.wantAdherence {
				t.Errorf("adherence = %v, want %v", rate, tt.wantAdherence)
			}
		})
	}
}

func TestMessengerType_Values(t *testing.T) {
	types := []user.MessengerType{
		user.MessengerTelegram,
		user.MessengerVK,
	}
	seen := make(map[user.MessengerType]bool)
	for _, mt := range types {
		if seen[mt] {
			t.Errorf("duplicate MessengerType: %q", mt)
		}
		seen[mt] = true
		if mt == "" {
			t.Error("MessengerType must not be empty")
		}
	}
}
