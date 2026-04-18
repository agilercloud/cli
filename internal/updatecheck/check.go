package updatecheck

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/agilercloud/cli/internal/selfupdate"
)

// Interval is the minimum time between background checks.
const Interval = 24 * time.Hour

// Options controls the background update check.
type Options struct {
	CmdName     string
	Version     string
	OutputMuted bool // true when --json or --quiet is active
	Fetch       func(ctx context.Context) (latestTag string, err error)
}

// Background prints a one-line notification if a known-newer release exists
// in the local cache, and asynchronously refreshes the cache if it's stale.
// Never blocks command execution.
func Background(opts Options) {
	if shouldSkip(opts.CmdName, opts.Version, opts.OutputMuted) {
		return
	}

	cache := Read()

	current := selfupdate.NormalizeVersion(opts.Version)
	latest := selfupdate.NormalizeVersion(cache.LatestVersion)
	if latest != "" && selfupdate.CompareVersions(current, latest) < 0 {
		fmt.Fprintf(os.Stderr, "a newer version of agiler is available: %s (run 'agiler upgrade')\n", cache.LatestVersion)
	}

	if time.Since(cache.CheckedAt) < Interval {
		return
	}

	go refresh(opts.Fetch)
}

func refresh(fetch func(ctx context.Context) (string, error)) {
	if fetch == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	tag, err := fetch(ctx)
	if err != nil {
		return
	}
	Write(Entry{
		CheckedAt:     time.Now(),
		LatestVersion: tag,
	})
}

func shouldSkip(cmdName, version string, outputMuted bool) bool {
	if os.Getenv("AGILER_NO_UPDATE_CHECK") != "" {
		return true
	}
	if selfupdate.NormalizeVersion(version) == "" {
		return true
	}
	switch cmdName {
	case "upgrade", "version", "help":
		return true
	}
	if outputMuted {
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
