package domain_unit

import (
	"testing"
	"time"
)

func TestTimesRoundtrip(t *testing.T) {
	// Симулируем roundtrip: time → string → time
	times := []time.Time{
		mustParseTime("08:00:00"),
		mustParseTime("12:30:00"),
		mustParseTime("20:00:00"),
	}

	// Convert to strings as the repo does
	strings := make([]string, len(times))
	for i, t := range times {
		strings[i] = t.Format("15:04:05")
	}

	// Convert back
	result := make([]time.Time, 0, len(strings))
	for _, s := range strings {
		t, err := time.Parse("15:04:05", s)
		if err != nil {
			continue
		}
		result = append(result, t)
	}

	if len(result) != len(times) {
		t.Fatalf("roundtrip: got %d times, want %d", len(result), len(times))
	}
	for i, want := range times {
		if result[i].Format("15:04:05") != want.Format("15:04:05") {
			t.Errorf("time[%d] = %q, want %q", i, result[i].Format("15:04:05"), want.Format("15:04:05"))
		}
	}
}

func TestTimesRoundtrip_Empty(t *testing.T) {
	var times []time.Time
	strings := make([]string, 0)
	result := make([]time.Time, 0)

	for _, s := range strings {
		tt, err := time.Parse("15:04:05", s)
		if err == nil {
			result = append(result, tt)
		}
	}

	if len(result) != len(times) {
		t.Fatalf("empty roundtrip: got %d, want %d", len(result), len(times))
	}
}

func TestTimesRoundtrip_InvalidStringSkipped(t *testing.T) {
	raw := []string{"08:00:00", "not-a-time", "20:00:00"}
	result := make([]time.Time, 0)
	for _, s := range raw {
		tt, err := time.Parse("15:04:05", s)
		if err == nil {
			result = append(result, tt)
		}
	}
	if len(result) != 2 {
		t.Errorf("expected 2 valid times, got %d", len(result))
	}
}

func mustParseTime(s string) time.Time {
	t, err := time.Parse("15:04:05", s)
	if err != nil {
		panic("mustParseTime: " + err.Error())
	}
	return t
}
