package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

const updateCheckInterval = 24 * time.Hour

type updateCheckCache struct {
	CheckedAt     time.Time `json:"checked_at"`
	LatestVersion string    `json:"latest_version"`
}

// backgroundUpdateCheck prints a one-line notification if a known-newer
// release exists in the local cache, and asynchronously refreshes the
// cache if it's stale. Never blocks command execution.
func backgroundUpdateCheck(cmdName string) {
	if shouldSkipUpdateCheck(cmdName) {
		return
	}

	cache := readUpdateCheckCache()

	current := normalizeVersion(Version)
	latest := normalizeVersion(cache.LatestVersion)
	if latest != "" && compareVersions(current, latest) < 0 {
		fmt.Fprintf(os.Stderr, "a newer version of agiler is available: %s (run 'agiler upgrade')\n", cache.LatestVersion)
	}

	if time.Since(cache.CheckedAt) < updateCheckInterval {
		return
	}

	go refreshUpdateCheckCache()
}

func refreshUpdateCheckCache() {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	rel, err := fetchRelease(ctx, "")
	if err != nil {
		return
	}
	writeUpdateCheckCache(updateCheckCache{
		CheckedAt:     time.Now(),
		LatestVersion: rel.TagName,
	})
}

func shouldSkipUpdateCheck(cmdName string) bool {
	if os.Getenv("AGILER_NO_UPDATE_CHECK") != "" {
		return true
	}
	if normalizeVersion(Version) == "" {
		return true
	}
	switch cmdName {
	case "upgrade", "version", "help":
		return true
	}
	if outputJSON || outputQuiet {
		return true
	}
	return !isStderrTTY()
}

func isStderrTTY() bool {
	info, err := os.Stderr.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeCharDevice != 0
}

func updateCheckPath() (string, error) {
	dir, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "agiler", "update-check.json"), nil
}

func readUpdateCheckCache() updateCheckCache {
	var c updateCheckCache
	path, err := updateCheckPath()
	if err != nil {
		return c
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return c
	}
	_ = json.Unmarshal(data, &c)
	return c
}

func writeUpdateCheckCache(c updateCheckCache) {
	path, err := updateCheckPath()
	if err != nil {
		return
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return
	}
	data, err := json.Marshal(c)
	if err != nil {
		return
	}
	_ = os.WriteFile(path, data, 0o600)
}
