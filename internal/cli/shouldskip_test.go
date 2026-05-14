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

	if !shouldSkip(fs, "/local.txt", 5, mtime) {
		t.Error("expected skip when size and mtime match")
	}
}

func TestShouldSkipSizeMismatch(t *testing.T) {
	fs := fsx.NewMemFS()
	mtime := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)
	fs.WriteFile("/local.txt", []byte("hello"), mtime)

	if shouldSkip(fs, "/local.txt", 99, mtime) {
		t.Error("did not expect skip on size mismatch")
	}
}

func TestShouldSkipMtimeMismatch(t *testing.T) {
	fs := fsx.NewMemFS()
	fs.WriteFile("/local.txt", []byte("hello"), time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC))
	other := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)

	if shouldSkip(fs, "/local.txt", 5, other) {
		t.Error("did not expect skip on mtime mismatch")
	}
}

func TestShouldSkipMissingLocal(t *testing.T) {
	fs := fsx.NewMemFS()
	mtime := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	if shouldSkip(fs, "/missing.txt", 5, mtime) {
		t.Error("did not expect skip when local file is missing")
	}
}

// TestShouldSkipSubSecondTimestamps verifies sub-second precision in the
// remote mtime doesn't trip up the comparison — the local FS only tracks
// seconds, so we match at second granularity.
func TestShouldSkipSubSecondTimestamps(t *testing.T) {
	fs := fsx.NewMemFS()
	mtime := time.Date(2025, 1, 1, 12, 0, 0, 123456789, time.UTC)
	fs.WriteFile("/local.txt", []byte("hello"), mtime)

	if !shouldSkip(fs, "/local.txt", 5, mtime) {
		t.Error("expected skip with sub-second timestamp")
	}
}
