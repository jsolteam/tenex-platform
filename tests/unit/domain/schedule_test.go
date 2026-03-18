package domain_unit

import (
	"testing"
	"time"

	"github.com/jsolteam/tenex-platform/internal/domain/schedule"
)

func TestDayOfWeek_Has(t *testing.T) {
	tests := []struct {
		name string
		mask schedule.DayOfWeek
		day  schedule.DayOfWeek
		want bool
	}{
		{"weekdays has monday", schedule.Weekdays, schedule.Monday, true},
		{"weekdays has friday", schedule.Weekdays, schedule.Friday, true},
		{"weekdays not saturday", schedule.Weekdays, schedule.Saturday, false},
		{"weekdays not sunday", schedule.Weekdays, schedule.Sunday, false},
		{"weekend has saturday", schedule.Weekend, schedule.Saturday, true},
		{"weekend has sunday", schedule.Weekend, schedule.Sunday, true},
		{"weekend not monday", schedule.Weekend, schedule.Monday, false},
		{"alldays has all", schedule.AllDays, schedule.Sunday, true},
		{"empty mask", 0, schedule.Monday, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.mask.Has(tt.day); got != tt.want {
				t.Errorf("DayOfWeek(%d).Has(%d) = %v, want %v", tt.mask, tt.day, got, tt.want)
			}
		})
	}
}

func TestSchedule_IsActive(t *testing.T) {
	today := time.Date(2025, 6, 15, 0, 0, 0, 0, time.UTC)
	yesterday := today.AddDate(0, 0, -1)
	tomorrow := today.AddDate(0, 0, 1)
	nextWeek := today.AddDate(0, 0, 7)

	endDate := today.AddDate(0, 0, 3)

	tests := []struct {
		name      string
		startDate time.Time
		endDate   *time.Time
		checkDate time.Time
		want      bool
	}{
		{"active today no end", today, nil, today, true},
		{"active after start no end", yesterday, nil, today, true},
		{"not started yet", tomorrow, nil, today, false},
		{"active within range", yesterday, &endDate, today, true},
		{"exactly on end date", yesterday, &today, today, true},
		{"after end date", yesterday, &yesterday, today, false},
		{"future schedule", tomorrow, &nextWeek, today, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &schedule.Schedule{
				StartDate: tt.startDate,
				EndDate:   tt.endDate,
			}
			if got := s.IsActive(tt.checkDate); got != tt.want {
				t.Errorf("IsActive(%v) = %v, want %v", tt.checkDate, got, tt.want)
			}
		})
	}
}

func TestAllDays_Value(t *testing.T) {
	// AllDays должен быть 127 = 1+2+4+8+16+32+64
	const wantAllDays = schedule.Monday | schedule.Tuesday | schedule.Wednesday |
		schedule.Thursday | schedule.Friday | schedule.Saturday | schedule.Sunday
	if schedule.AllDays != wantAllDays {
		t.Errorf("AllDays = %d, want %d", schedule.AllDays, wantAllDays)
	}
}
