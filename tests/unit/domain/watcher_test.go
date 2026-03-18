package domain_unit

import (
	"testing"
	"time"

	"github.com/jsolteam/tenex-platform/internal/domain/watcher"
)

func TestWatcherNotification_IsSent(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name   string
		sentAt *time.Time
		want   bool
	}{
		{"not sent", nil, false},
		{"sent", &now, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			n := &watcher.WatcherNotification{SentAt: tt.sentAt}
			if got := n.IsSent(); got != tt.want {
				t.Errorf("IsSent() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNotificationEvent_Values(t *testing.T) {
	events := []watcher.NotificationEvent{
		watcher.EventConfirmed,
		watcher.EventSkipped,
		watcher.EventPostponed,
	}
	seen := make(map[watcher.NotificationEvent]bool)
	for _, e := range events {
		if seen[e] {
			t.Errorf("duplicate NotificationEvent: %q", e)
		}
		seen[e] = true
		if e == "" {
			t.Error("NotificationEvent must not be empty")
		}
	}
}
