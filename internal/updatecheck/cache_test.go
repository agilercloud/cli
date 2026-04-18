package updatecheck

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// isolateCache sets HOME and XDG_CACHE_HOME so both macOS and Linux
// write into the provided temp directory.
func isolateCache(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", dir)
	t.Setenv("HOME", dir)
}

// cacheFile returns the resolved cache file path for the current env.
func cacheFile(t *testing.T) string {
	t.Helper()
	p, err := path()
	if err != nil {
		t.Fatalf("cache path: %v", err)
	}
	return p
}

func TestCacheRoundtrip(t *testing.T) {
	isolateCache(t)

	want := Entry{
		CheckedAt:     time.Date(2024, 1, 2, 3, 4, 5, 0, time.UTC),
		LatestVersion: "v1.2.3",
	}
	Write(want)

	got := Read()
	if !got.CheckedAt.Equal(want.CheckedAt) {
		t.Errorf("CheckedAt: got %v, want %v", got.CheckedAt, want.CheckedAt)
	}
	if got.LatestVersion != want.LatestVersion {
		t.Errorf("LatestVersion: got %q, want %q", got.LatestVersion, want.LatestVersion)
	}
}

func TestReadMissingReturnsZero(t *testing.T) {
	isolateCache(t)

	got := Read()
	if !got.CheckedAt.IsZero() || got.LatestVersion != "" {
		t.Errorf("expected zero Entry from missing file, got %+v", got)
	}
}

func TestReadCorruptReturnsZero(t *testing.T) {
	isolateCache(t)

	p := cacheFile(t)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte("not json"), 0o600); err != nil {
		t.Fatal(err)
	}

	got := Read()
	if !got.CheckedAt.IsZero() || got.LatestVersion != "" {
		t.Errorf("expected zero Entry from corrupt file, got %+v", got)
	}

	// Sanity: valid data still parses after Write overwrites the corrupt file.
	Write(Entry{LatestVersion: "v9.9.9"})
	if got := Read(); got.LatestVersion != "v9.9.9" {
		t.Errorf("expected v9.9.9 after overwrite, got %q", got.LatestVersion)
	}
}
