package medicine

import "time"

type Medicine struct {
	ID           int64
	UserID       int64
	Name         string
	PhotoMediaID *int64
	CreatedAt    time.Time
	UpdatedAt    time.Time
}
