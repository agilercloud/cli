package main

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"
)

const (
	ghRepo         = "agilercloud/cli"
	maxDownloadMiB = 50
)

// ghBaseURL is the GitHub API host; overridable in tests.
var ghBaseURL = "https://api.github.com"

type release struct {
	TagName string `json:"tag_name"`
	HTMLURL string `json:"html_url"`
}

type installSource int

const (
	sourceSelfManaged installSource = iota
	sourceHomebrew
	sourceGoInstall
	sourceDev
)

// detectInstallSource classifies how the running binary was installed.
// Returns the source and an upgrade hint for the user when we refuse.
func detectInstallSource(exe, version string) (installSource, string) {
	if normalizeVersion(version) == "" {
		return sourceDev, "this is a dev build; use --force to replace with the latest release"
	}

	brewMarkers := []string{"/Cellar/", "/Caskroom/", "/opt/homebrew/", "/home/linuxbrew/", "/.linuxbrew/"}
	for _, m := range brewMarkers {
		if strings.Contains(exe, m) {
			return sourceHomebrew, fmt.Sprintf("agiler was installed via Homebrew (%s)\nupgrade with: brew upgrade --cask agiler", exe)
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
			return sourceGoInstall, fmt.Sprintf("agiler was installed via 'go install' (%s)\nupgrade with: go install github.com/agilercloud/cli/cmd/agiler@latest", exe)
		}
	}

	return sourceSelfManaged, ""
}

// normalizeVersion strips a leading 'v' and any -pre-release or +build suffix.
// Returns "" for dev builds.
func normalizeVersion(v string) string {
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

// compareVersions returns -1, 0, or 1 comparing semver-lite strings.
// Empty string sorts before any concrete version (dev < released).
func compareVersions(a, b string) int {
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

// archiveName mirrors .goreleaser.yml's name_template.
func archiveName(version, goos, goarch string) string {
	arch := goarch
	switch goarch {
	case "amd64":
		arch = "x86_64"
	}
	return fmt.Sprintf("agiler_%s_%s_%s.tar.gz", strings.TrimPrefix(version, "v"), goos, arch)
}

func fetchRelease(ctx context.Context, tag string) (*release, error) {
	path := "releases/latest"
	if tag != "" {
		path = "releases/tags/" + tag
	}
	url := fmt.Sprintf("%s/repos/%s/%s", ghBaseURL, ghRepo, path)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "agiler-cli")
	if tok := os.Getenv("GITHUB_TOKEN"); tok != "" {
		req.Header.Set("Authorization", "Bearer "+tok)
	}

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("could not reach github.com: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == 404 {
		if tag != "" {
			return nil, fmt.Errorf("release %s not found", tag)
		}
		return nil, errors.New("no releases published yet")
	}
	if resp.StatusCode == 403 || resp.StatusCode == 429 {
		return nil, errors.New("github rate limit exceeded; set GITHUB_TOKEN to increase the limit")
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("github responded %s", resp.Status)
	}

	var r release
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&r); err != nil {
		return nil, fmt.Errorf("invalid release response: %w", err)
	}
	if r.TagName == "" {
		return nil, errors.New("release response missing tag_name")
	}
	return &r, nil
}

// downloadAndVerify fetches the archive and checksums.txt, verifies SHA-256,
// and returns the path to the downloaded archive on success.
func downloadAndVerify(ctx context.Context, tag, archive, tmpDir string) (string, error) {
	base := fmt.Sprintf("https://github.com/%s/releases/download/%s", ghRepo, tag)
	archiveURL := base + "/" + archive
	sumsURL := base + "/checksums.txt"

	archivePath := filepath.Join(tmpDir, archive)
	sumsPath := filepath.Join(tmpDir, "checksums.txt")

	if err := downloadToFile(ctx, archiveURL, archivePath); err != nil {
		return "", fmt.Errorf("download %s: %w", archive, err)
	}
	if err := downloadToFile(ctx, sumsURL, sumsPath); err != nil {
		return "", fmt.Errorf("download checksums.txt: %w", err)
	}

	sumsBytes, err := os.ReadFile(sumsPath)
	if err != nil {
		return "", err
	}
	expected, err := checksumLookup(sumsBytes, archive)
	if err != nil {
		return "", err
	}

	actual, err := sha256File(archivePath)
	if err != nil {
		return "", err
	}
	if subtle.ConstantTimeCompare([]byte(expected), []byte(actual)) != 1 {
		return "", fmt.Errorf("checksum mismatch for %s; refusing to install", archive)
	}
	return archivePath, nil
}

func downloadToFile(ctx context.Context, url, dst string) error {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "agiler-cli")

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return fmt.Errorf("status %s", resp.Status)
	}

	f, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer f.Close()

	limit := int64(maxDownloadMiB) << 20
	if _, err := io.Copy(f, io.LimitReader(resp.Body, limit+1)); err != nil {
		return err
	}
	info, err := f.Stat()
	if err != nil {
		return err
	}
	if info.Size() > limit {
		return fmt.Errorf("download exceeded %d MiB limit", maxDownloadMiB)
	}
	return nil
}

func sha256File(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// checksumLookup parses a `sha256  filename` file and returns the hash for archive.
func checksumLookup(sumsTxt []byte, archive string) (string, error) {
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

// extractBinary pulls the "agiler" regular-file entry out of a .tar.gz.
func extractBinary(tgzPath, destPath string) error {
	f, err := os.Open(tgzPath)
	if err != nil {
		return err
	}
	defer f.Close()

	gz, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	defer gz.Close()

	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			return errors.New("archive did not contain agiler binary")
		}
		if err != nil {
			return err
		}
		if hdr.Typeflag != tar.TypeReg {
			continue
		}
		if filepath.Base(hdr.Name) != "agiler" {
			continue
		}
		out, err := os.OpenFile(destPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
		if err != nil {
			return err
		}
		if _, err := io.Copy(out, io.LimitReader(tr, int64(maxDownloadMiB)<<20)); err != nil {
			out.Close()
			return err
		}
		return out.Close()
	}
}

// replaceExecutable swaps the new binary into place atomically.
// Works while the target is currently executing (POSIX-only).
func replaceExecutable(newPath, targetPath string) error {
	if runtime.GOOS == "windows" {
		return errors.New("self-update not supported on windows")
	}
	if err := os.Chmod(newPath, 0o755); err != nil {
		return err
	}
	if runtime.GOOS == "darwin" {
		// Best-effort: strip quarantine if present. Ignore errors.
		_ = exec.Command("/usr/bin/xattr", "-d", "com.apple.quarantine", newPath).Run()
	}
	if err := os.Rename(newPath, targetPath); err == nil {
		return nil
	} else if !errors.Is(err, syscall.EXDEV) {
		return err
	}
	// Different-filesystem fallback: stage a sibling, then rename.
	siblingPath := targetPath + ".new"
	if err := copyFile(newPath, siblingPath); err != nil {
		return err
	}
	if err := os.Chmod(siblingPath, 0o755); err != nil {
		os.Remove(siblingPath)
		return err
	}
	if err := os.Rename(siblingPath, targetPath); err != nil {
		os.Remove(siblingPath)
		return err
	}
	return nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}

// preflightWritable fails fast if the user can't replace the target binary.
func preflightWritable(targetPath string) error {
	f, err := os.OpenFile(targetPath, os.O_WRONLY|os.O_APPEND, 0)
	if err != nil {
		if errors.Is(err, os.ErrPermission) {
			return fmt.Errorf("cannot write to %s: permission denied\ntry: sudo agiler upgrade", targetPath)
		}
		return err
	}
	return f.Close()
}

// resolveExecutable returns the real path of the running binary,
// with symlinks resolved.
func resolveExecutable() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	resolved, err := filepath.EvalSymlinks(exe)
	if err != nil {
		return exe, nil //nolint:nilerr // fall back to the unresolved path
	}
	return resolved, nil
}
