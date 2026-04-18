package selfupdate

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

// DownloadAndVerify fetches the archive and checksums.txt from the GitHub
// release for tag, verifies SHA-256, and returns the path to the downloaded
// archive on success.
func DownloadAndVerify(ctx context.Context, tag, archive, tmpDir string) (string, error) {
	base := fmt.Sprintf("https://github.com/%s/releases/download/%s", Repo, tag)
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
	expected, err := ChecksumLookup(sumsBytes, archive)
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

	limit := int64(MaxDownloadMiB) << 20
	if _, err := io.Copy(f, io.LimitReader(resp.Body, limit+1)); err != nil {
		return err
	}
	info, err := f.Stat()
	if err != nil {
		return err
	}
	if info.Size() > limit {
		return fmt.Errorf("download exceeded %d MiB limit", MaxDownloadMiB)
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

// ExtractBinary pulls the "agiler" regular-file entry out of a .tar.gz.
func ExtractBinary(tgzPath, destPath string) error {
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
		if _, err := io.Copy(out, io.LimitReader(tr, int64(MaxDownloadMiB)<<20)); err != nil {
			out.Close()
			return err
		}
		return out.Close()
	}
}
