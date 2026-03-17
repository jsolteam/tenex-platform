package media

import "errors"

var (
	ErrNotFound = errors.New("media: not found")

	ErrVariantNotFound = errors.New("media: variant not found")

	ErrInvalidMediaType = errors.New("media: invalid media type")

	ErrInvalidStorageType = errors.New("media: invalid storage type")
)
