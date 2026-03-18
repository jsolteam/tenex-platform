package domain_unit

import (
	"testing"

	"github.com/jsolteam/tenex-platform/internal/domain/reminder"
)

func TestReminder_IsFinal(t *testing.T) {
	tests := []struct {
		status reminder.Status
		want   bool
	}{
		{reminder.StatusPending, false},
		{reminder.StatusSent, false},
		{reminder.StatusPostponed, false},
		{reminder.StatusConfirmed, true},
		{reminder.StatusSkipped, true},
		{reminder.StatusExpired, true},
	}
	for _, tt := range tests {
		t.Run(string(tt.status), func(t *testing.T) {
			r := &reminder.Reminder{Status: tt.status}
			if got := r.IsFinal(); got != tt.want {
				t.Errorf("IsFinal() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestReminder_CanRetry(t *testing.T) {
	tests := []struct {
		name       string
		status     reminder.Status
		retryCount int16
		maxRetries int
		want       bool
	}{
		{"pending within limit", reminder.StatusPending, 0, 3, true},
		{"pending at limit", reminder.StatusPending, 3, 3, false},
		{"pending over limit", reminder.StatusPending, 4, 3, false},
		{"sent within limit", reminder.StatusSent, 1, 3, true},
		{"confirmed is final", reminder.StatusConfirmed, 0, 3, false},
		{"skipped is final", reminder.StatusSkipped, 0, 3, false},
		{"expired is final", reminder.StatusExpired, 0, 3, false},
		{"postponed can retry", reminder.StatusPostponed, 0, 3, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &reminder.Reminder{
				Status:     tt.status,
				RetryCount: tt.retryCount,
			}
			if got := r.CanRetry(tt.maxRetries); got != tt.want {
				t.Errorf("CanRetry(%d) = %v, want %v", tt.maxRetries, got, tt.want)
			}
		})
	}
}
