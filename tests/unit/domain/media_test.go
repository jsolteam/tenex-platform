package domain_unit

import (
	"testing"

	"github.com/jsolteam/tenex-platform/internal/domain/media"
)

func TestMediaType_Values(t *testing.T) {
	types := []media.MediaType{
		media.MediaTypePhoto,
		media.MediaTypeVideo,
		media.MediaTypeDocument,
	}
	seen := make(map[media.MediaType]bool)
	for _, mt := range types {
		if seen[mt] {
			t.Errorf("duplicate MediaType: %q", mt)
		}
		seen[mt] = true
		if mt == "" {
			t.Error("MediaType must not be empty")
		}
	}
}

func TestStorageType_Values(t *testing.T) {
	types := []media.StorageType{
		media.StorageTypeMessenger,
		media.StorageTypeS3,
		media.StorageTypeURL,
	}
	seen := make(map[media.StorageType]bool)
	for _, st := range types {
		if seen[st] {
			t.Errorf("duplicate StorageType: %q", st)
		}
		seen[st] = true
		if st == "" {
			t.Error("StorageType must not be empty")
		}
	}
}

func TestMediaVariant_MessengerOptional(t *testing.T) {
	// Для S3 и URL поле Messenger пустое — это нормально
	v := media.MediaVariant{
		StorageType: media.StorageTypeS3,
		ExternalID:  "bucket/key.jpg",
		// Messenger — пустая строка
	}
	if v.Messenger != "" {
		t.Error("Messenger should default to empty for S3")
	}
}
