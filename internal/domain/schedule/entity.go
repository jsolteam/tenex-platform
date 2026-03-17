package schedule

import "time"

type ScheduleType string

const (
	// ScheduleTypeDaily — каждый день в указанное время.
	ScheduleTypeDaily ScheduleType = "daily"

	// ScheduleTypeWeekly — в определённые дни недели (bitmask).
	ScheduleTypeWeekly ScheduleType = "weekly"

	// ScheduleTypeInterval — каждые N дней.
	ScheduleTypeInterval ScheduleType = "interval"
)

// DayOfWeek — bitmask дней недели.
//
//	1  = Monday
//	2  = Tuesday
//	4  = Wednesday
//	8  = Thursday
//	16 = Friday
//	32 = Saturday
//	64 = Sunday
type DayOfWeek int16

const (
	Monday    DayOfWeek = 1
	Tuesday   DayOfWeek = 2
	Wednesday DayOfWeek = 4
	Thursday  DayOfWeek = 8
	Friday    DayOfWeek = 16
	Saturday  DayOfWeek = 32
	Sunday    DayOfWeek = 64

	Weekdays DayOfWeek = Monday | Tuesday | Wednesday | Thursday | Friday
	Weekend  DayOfWeek = Saturday | Sunday
	AllDays  DayOfWeek = Weekdays | Weekend
)

// Has проверяет что день входит в bitmask.
func (d DayOfWeek) Has(day DayOfWeek) bool {
	return d&day != 0
}

type Schedule struct {
	ID           int64
	MedicineID   int64
	Type         ScheduleType
	IntervalDays *int16      // только для ScheduleTypeInterval
	DaysOfWeek   *DayOfWeek  // только для ScheduleTypeWeekly
	Times        []time.Time // время приёма (только часы и минуты)
	StartDate    time.Time
	EndDate      *time.Time
	CreatedAt    time.Time
}

func (s *Schedule) IsActive(date time.Time) bool {
	d := date.Truncate(24 * time.Hour)
	start := s.StartDate.Truncate(24 * time.Hour)
	if d.Before(start) {
		return false
	}
	if s.EndDate != nil {
		end := s.EndDate.Truncate(24 * time.Hour)
		if d.After(end) {
			return false
		}
	}
	return true
}
