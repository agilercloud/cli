package cli

import (
	"testing"
	"time"

	"github.com/agilercloud/cli/internal/fsx"
)

func TestShouldSkipMatchingSizeAndMtime(t *testing.T) {
	fs := fsx.NewMemFS()
	mtime := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)
	fs.WriteFile("/local.txt", []byte("hello"), mtime)

	if !shouldSkip(fs, "/local.txt", 5, mtime.Format(time.RFC3339)) {
		t.Error("expected skip when size and mtime match")
	}
}

func TestShouldSkipSizeMismatch(t *testing.T) {
	fs := fsx.NewMemFS()
	mtime := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)
	fs.WriteFile("/local.txt", []byte("hello"), mtime)

	if shouldSkip(fs, "/local.txt", 99, mtime.Format(time.RFC3339)) {
		t.Error("did not expect skip on size mismatch")
	}
}

func TestShouldSkipMtimeMismatch(t *testing.T) {
	fs := fsx.NewMemFS()
	fs.WriteFile("/local.txt", []byte("hello"), time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC))
	other := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)

	if shouldSkip(fs, "/local.txt", 5, other.Format(time.RFC3339)) {
		t.Error("did not expect skip on mtime mismatch")
	}
}

func TestShouldSkipMissingLocal(t *testing.T) {
	fs := fsx.NewMemFS()
	if shouldSkip(fs, "/missing.txt", 5, "2025-01-01T00:00:00Z") {
		t.Error("did not expect skip when local file is missing")
	}
}

func TestShouldSkipRFC3339Nano(t *testing.T) {
	fs := fsx.NewMemFS()
	mtime := time.Date(2025, 1, 1, 12, 0, 0, 123456789, time.UTC)
	fs.WriteFile("/local.txt", []byte("hello"), mtime)

	// RFC3339Nano format with fractional seconds — should still match at second granularity.
	if !shouldSkip(fs, "/local.txt", 5, mtime.Format(time.RFC3339Nano)) {
		t.Error("expected skip with RFC3339Nano format")
	}
}

func TestShouldSkipInvalidTime(t *testing.T) {
	fs := fsx.NewMemFS()
	fs.WriteFile("/local.txt", []byte("hello"), time.Now())
	if shouldSkip(fs, "/local.txt", 5, "not-a-time") {
		t.Error("did not expect skip with unparseable remote mtime")
	}
}
