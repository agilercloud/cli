// Package updatecheck handles the background check for a newer agiler release.
package updatecheck

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

// Entry is the on-disk cache of the last successful update check.
type Entry struct {
	CheckedAt     time.Time `json:"checked_at"`
	LatestVersion string    `json:"latest_version"`
}

// path returns the platform-specific path to the cache file.
func path() (string, error) {
	dir, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "agiler", "update-check.json"), nil
}

// Read returns the cached entry, or a zero value if reading fails.
func Read() Entry {
	var c Entry
	p, err := path()
	if err != nil {
		return c
	}
	data, err := os.ReadFile(p)
	if err != nil {
		return c
	}
	_ = json.Unmarshal(data, &c)
	return c
}

// Write persists the entry to the cache file on a best-effort basis.
func Write(c Entry) {
	p, err := path()
	if err != nil {
		return
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return
	}
	data, err := json.Marshal(c)
	if err != nil {
		return
	}
	_ = os.WriteFile(p, data, 0o600)
}
