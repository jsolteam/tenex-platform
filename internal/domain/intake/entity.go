package intake

import "time"

type Status string

const (
	// StatusConfirmed — пользователь принял лекарство.
	StatusConfirmed Status = "confirmed"

	// StatusSkipped — пользователь пропустил приём (с указанием причины).
	StatusSkipped Status = "skipped"
)

type Intake struct {
	ID           int64
	ReminderID   int64 // логический FK — физический FK отсутствует из-за партиционирования
	Status       Status
	ProofMediaID *int64 // фото/видео подтверждения
	Reason       string // причина пропуска, заполняется только при StatusSkipped
	CreatedAt    time.Time
}
