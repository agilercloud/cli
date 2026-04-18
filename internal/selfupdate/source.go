// Package selfupdate implements the agiler CLI self-upgrade flow.
//
// This file contains pure helpers used by the upgrade command and by the
// background update-check goroutine. They do not perform I/O and can be
// tested in isolation.
package selfupdate

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// InstallSource classifies how the running binary was installed.
type InstallSource int

const (
	SourceSelfManaged InstallSource = iota
	SourceHomebrew
	SourceGoInstall
	SourceDev
)

// DetectInstallSource classifies how the running binary was installed.
// Returns the source and an upgrade hint for the user when we refuse.
func DetectInstallSource(exe, version string) (InstallSource, string) {
	if NormalizeVersion(version) == "" {
		return SourceDev, "this is a dev build; use --force to replace with the latest release"
	}

	brewMarkers := []string{"/Cellar/", "/Caskroom/", "/opt/homebrew/", "/home/linuxbrew/", "/.linuxbrew/"}
	for _, m := range brewMarkers {
		if strings.Contains(exe, m) {
			return SourceHomebrew, fmt.Sprintf("agiler was installed via Homebrew (%s)\nupgrade with: brew upgrade --cask agiler", exe)
		}
	}

	goBins := []string{}
	if v := os.Getenv("GOBIN"); v != "" {
		goBins = append(goBins, filepath.Clean(v))
	}
	if v := os.Getenv("GOPATH"); v != "" {
		for _, p := range filepath.SplitList(v) {
			goBins = append(goBins, filepath.Join(p, "bin"))
		}
	}
	if home, err := os.UserHomeDir(); err == nil {
		goBins = append(goBins, filepath.Join(home, "go", "bin"))
	}
	for _, b := range goBins {
		if b != "" && strings.HasPrefix(exe, b+string(os.PathSeparator)) {
			return SourceGoInstall, fmt.Sprintf("agiler was installed via 'go install' (%s)\nupgrade with: go install github.com/agilercloud/cli/cmd/agiler@latest", exe)
		}
	}

	return SourceSelfManaged, ""
}

// NormalizeVersion strips a leading 'v' and any -pre-release or +build suffix.
// Returns "" for dev builds.
func NormalizeVersion(v string) string {
	v = strings.TrimSpace(v)
	v = strings.TrimPrefix(v, "v")
	if v == "" || v == "dev" {
		return ""
	}
	if i := strings.IndexAny(v, "-+"); i >= 0 {
		v = v[:i]
	}
	return v
}

// CompareVersions returns -1, 0, or 1 comparing semver-lite strings.
// Empty string sorts before any concrete version (dev < released).
func CompareVersions(a, b string) int {
	if a == b {
		return 0
	}
	if a == "" {
		return -1
	}
	if b == "" {
		return 1
	}
	ap := splitVersion(a)
	bp := splitVersion(b)
	for i := 0; i < 3; i++ {
		if ap[i] < bp[i] {
			return -1
		}
		if ap[i] > bp[i] {
			return 1
		}
	}
	return 0
}

func splitVersion(v string) [3]int {
	var out [3]int
	parts := strings.SplitN(v, ".", 3)
	for i := 0; i < 3 && i < len(parts); i++ {
		n, _ := strconv.Atoi(parts[i])
		out[i] = n
	}
	return out
}

// ArchiveName mirrors .goreleaser.yml's name_template.
func ArchiveName(version, goos, goarch string) string {
	arch := goarch
	switch goarch {
	case "amd64":
		arch = "x86_64"
	}
	return fmt.Sprintf("agiler_%s_%s_%s.tar.gz", strings.TrimPrefix(version, "v"), goos, arch)
}

// ChecksumLookup parses a `sha256  filename` file and returns the hash for archive.
func ChecksumLookup(sumsTxt []byte, archive string) (string, error) {
	for _, line := range strings.Split(string(sumsTxt), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		if fields[len(fields)-1] == archive {
			return fields[0], nil
		}
	}
	return "", fmt.Errorf("checksum for %s not found in checksums.txt", archive)
}
