package media

import "time"

type MediaType string

const (
	MediaTypePhoto    MediaType = "photo"
	MediaTypeVideo    MediaType = "video"
	MediaTypeDocument MediaType = "document"
)

type StorageType string

const (
	StorageTypeMessenger StorageType = "messenger" // FileID в мессенджере
	StorageTypeS3        StorageType = "s3"        // ключ в S3/MinIO
	StorageTypeURL       StorageType = "url"       // публичный URL
)

const OwnerSystemID int64 = -1

type Media struct {
	ID          int64
	OwnerUserID int64
	MediaType   MediaType
	CreatedAt   time.Time
}

type MediaVariant struct {
	ID          int64
	MediaID     int64
	StorageType StorageType
	Messenger   string // Только для StorageTypeMessenger
	ExternalID  string // FileID, S3 key и т.д.
	URL         string // публичный URL
	CreatedAt   time.Time
}
