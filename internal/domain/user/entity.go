package user

import "time"

type User struct {
	ID        int64
	Timezone  string
	Language  string
	CreatedAt time.Time
	UpdatedAt time.Time
}

type MessengerType string

const (
	MessengerTelegram MessengerType = "telegram"
	MessengerVK       MessengerType = "vk"
)

type UserContact struct {
	ID              int64
	UserID          int64
	MessengerType   MessengerType
	MessengerUserID string
	Username        string
	IsPrimary       bool
	CreatedAt       time.Time
}

type UserStatistics struct {
	UserID         int64
	TotalReminders int
	Confirmed      int
	Skipped        int
	AdherenceRate  float64 // confirmed / total_reminders [0..1]
	UpdatedAt      time.Time
}
