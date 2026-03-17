package schedule

import "errors"

var (
	ErrNotFound = errors.New("schedule: not found")

	ErrEmptyTimes = errors.New("schedule: at least one time is required")

	ErrInvalidType = errors.New("schedule: invalid schedule type")

	ErrIntervalRequired = errors.New("schedule: interval_days is required for interval type")

	ErrDaysOfWeekRequired = errors.New("schedule: days_of_week is required for weekly type")
	
	ErrInvalidDateRange = errors.New("schedule: end_date cannot be before start_date")
)
