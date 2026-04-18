package cli

import (
	"testing"
	"time"
)

func TestParseTimeFlag(t *testing.T) {
	now := time.Date(2025, 3, 15, 12, 0, 0, 0, time.UTC)

	// RFC3339 absolute
	got, err := parseTimeFlag("2025-01-01T10:00:00Z", now)
	if err != nil {
		t.Fatalf("RFC3339: %v", err)
	}
	want := time.Date(2025, 1, 1, 10, 0, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Errorf("RFC3339 got %v, want %v", got, want)
	}

	// Duration relative to now
	got, err = parseTimeFlag("1h", now)
	if err != nil {
		t.Fatalf("1h: %v", err)
	}
	if !got.Equal(now.Add(-time.Hour)) {
		t.Errorf("1h relative to %v gave %v", now, got)
	}

	// Invalid
	if _, err := parseTimeFlag("nope", now); err == nil {
		t.Error("expected error for garbage input")
	}
}
