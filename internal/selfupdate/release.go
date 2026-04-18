package selfupdate

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

const (
	// Repo is the GitHub owner/repo slug releases are fetched from.
	Repo = "agilercloud/cli"
	// MaxDownloadMiB caps the size of an archive or checksums file.
	MaxDownloadMiB = 50
)

// BaseURL is the GitHub API host; overridable in tests.
var BaseURL = "https://api.github.com"

// Release is a minimal projection of GitHub's release JSON.
type Release struct {
	TagName string `json:"tag_name"`
	HTMLURL string `json:"html_url"`
}

// FetchRelease fetches either the latest release ("") or a specific tag.
func FetchRelease(ctx context.Context, tag string) (*Release, error) {
	path := "releases/latest"
	if tag != "" {
		path = "releases/tags/" + tag
	}
	url := fmt.Sprintf("%s/repos/%s/%s", BaseURL, Repo, path)

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

	var r Release
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&r); err != nil {
		return nil, fmt.Errorf("invalid release response: %w", err)
	}
	if r.TagName == "" {
		return nil, errors.New("release response missing tag_name")
	}
	return &r, nil
}
