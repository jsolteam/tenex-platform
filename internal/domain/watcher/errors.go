package watcher

import "errors"

var (
	ErrNotFound = errors.New("watcher: not found")

	ErrAlreadyExists = errors.New("watcher: already exists")

	ErrSelfWatch = errors.New("watcher: user cannot watch themselves")

	ErrNotificationNotFound = errors.New("watcher: notification not found")

	ErrNotificationAlreadySent = errors.New("watcher: notification already sent")
	
	ErrInvalidEvent = errors.New("watcher: invalid notification event")
)
