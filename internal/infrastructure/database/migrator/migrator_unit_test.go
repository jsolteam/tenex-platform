package migrator_test

import (
	"testing"

	"github.com/jsolteam/tenex-platform/internal/infrastructure/database/migrator"
)

func TestParseFilename(t *testing.T) {
	tests := []struct {
		input       string
		wantVersion int
		wantName    string
		wantErr     bool
	}{
		{"0001_create_users.sql", 1, "create_users", false},
		{"0008_create_statistics_and_config.sql", 8, "create_statistics_and_config", false},
		{"0042_some_migration.sql", 42, "some_migration", false},
		{"no_number.sql", 0, "", true},
		{"0000_zero.sql", 0, "", true}, // version must be > 0
		{"abc_name.sql", 0, "", true},
		{"onlyone.sql", 0, "", true},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			v, name, err := migrator.ParseFilename(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error for %q, got nil", tt.input)
				}
				return
			}
			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}
			if v != tt.wantVersion {
				t.Errorf("version = %d, want %d", v, tt.wantVersion)
			}
			if name != tt.wantName {
				t.Errorf("name = %q, want %q", name, tt.wantName)
			}
		})
	}
}

func TestChecksum_Deterministic(t *testing.T) {
	content := []byte("SELECT 1;")
	c1 := migrator.Checksum(content)
	c2 := migrator.Checksum(content)
	if c1 != c2 {
		t.Errorf("checksum not deterministic: %q != %q", c1, c2)
	}
	if c1 == "" {
		t.Error("checksum must not be empty")
	}
}

func TestChecksum_DifferentContent(t *testing.T) {
	c1 := migrator.Checksum([]byte("SELECT 1;"))
	c2 := migrator.Checksum([]byte("SELECT 2;"))
	if c1 == c2 {
		t.Error("different content must produce different checksums")
	}
}

func TestDetectDuplicates(t *testing.T) {
	tests := []struct {
		name       string
		migrations []migrator.Migration
		wantErr    bool
	}{
		{
			"no duplicates",
			[]migrator.Migration{
				{Version: 1, Filename: "0001_a.sql"},
				{Version: 2, Filename: "0002_b.sql"},
				{Version: 3, Filename: "0003_c.sql"},
			},
			false,
		},
		{
			"duplicate version",
			[]migrator.Migration{
				{Version: 1, Filename: "0001_a.sql"},
				{Version: 1, Filename: "0001_b.sql"},
			},
			true,
		},
		{
			"empty list",
			[]migrator.Migration{},
			false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := migrator.DetectDuplicates(tt.migrations)
			if tt.wantErr && err == nil {
				t.Error("expected error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}
