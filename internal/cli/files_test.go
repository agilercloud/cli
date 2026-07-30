package cli

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/agilercloud/cli/internal/api"
)

// TestFilesMoveSendsXMoveSourceHeader verifies the move command issues a PUT
// against the destination URL with X-Move-Source carrying the project-relative
// source path, and no JSON body.
func TestFilesMoveSendsXMoveSourceHeader(t *testing.T) {
	var (
		gotMethod  string
		gotPath    string
		gotMove    string
		gotCopy    string
		gotINM     string
		gotCT      string
		gotBodyLen int64
	)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		gotMove = r.Header.Get("X-Move-Source")
		gotCopy = r.Header.Get("X-Copy-Source")
		gotINM = r.Header.Get("If-None-Match")
		gotCT = r.Header.Get("Content-Type")
		gotBodyLen = r.ContentLength
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"name":"b.txt","path":"docs/b.txt"}`))
	}))
	t.Cleanup(srv.Close)

	a, _, _ := newTestApp(t)
	a.API = api.NewClient(srv.URL, "test-key", api.Options{})

	cmd := newFilesMoveCmd(a)
	cmd.SetArgs([]string{"docs/a.txt", "docs/b.txt", "--overwrite"})
	if err := cmd.ExecuteContext(context.Background()); err != nil {
		t.Fatalf("move command failed: %v", err)
	}

	if gotMethod != http.MethodPut {
		t.Errorf("method = %q, want PUT", gotMethod)
	}
	if gotPath != "/v1/projects/proj-1/files/docs/b.txt" {
		t.Errorf("path = %q", gotPath)
	}
	if gotMove != "docs/a.txt" {
		t.Errorf("X-Move-Source = %q", gotMove)
	}
	if gotCopy != "" {
		t.Errorf("X-Copy-Source should be empty, got %q", gotCopy)
	}
	if gotINM != "" {
		t.Errorf("If-None-Match should be unset with --overwrite, got %q", gotINM)
	}
	if gotCT != "" {
		// Content-Type must NOT be set on a body-less move/copy PUT.
		t.Errorf("Content-Type should be unset for move, got %q", gotCT)
	}
	if gotBodyLen > 0 {
		t.Errorf("body length = %d, want 0", gotBodyLen)
	}
}

// TestFilesCopySendsXCopySourceHeader mirrors the move test for copy.
func TestFilesCopySendsXCopySourceHeader(t *testing.T) {
	var gotCopy string
	var gotMove string
	var gotINM string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotCopy = r.Header.Get("X-Copy-Source")
		gotMove = r.Header.Get("X-Move-Source")
		gotINM = r.Header.Get("If-None-Match")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"name":"b.txt","path":"docs/b.txt"}`))
	}))
	t.Cleanup(srv.Close)

	a, _, _ := newTestApp(t)
	a.API = api.NewClient(srv.URL, "test-key", api.Options{})

	cmd := newFilesCopyCmd(a)
	cmd.SetArgs([]string{"docs/a.txt", "docs/b.txt", "--overwrite"})
	if err := cmd.ExecuteContext(context.Background()); err != nil {
		t.Fatalf("copy command failed: %v", err)
	}

	if gotCopy != "docs/a.txt" {
		t.Errorf("X-Copy-Source = %q", gotCopy)
	}
	if gotMove != "" {
		t.Errorf("X-Move-Source should be empty, got %q", gotMove)
	}
	if gotINM != "" {
		t.Errorf("If-None-Match should be unset with --overwrite, got %q", gotINM)
	}
}

// TestFilesMoveDefaultSetsIfNoneMatch verifies that without --overwrite, move
// refuses to clobber an existing destination by sending If-None-Match: *.
func TestFilesMoveDefaultSetsIfNoneMatch(t *testing.T) {
	var gotINM string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotINM = r.Header.Get("If-None-Match")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{}`))
	}))
	t.Cleanup(srv.Close)

	a, _, _ := newTestApp(t)
	a.API = api.NewClient(srv.URL, "test-key", api.Options{})

	cmd := newFilesMoveCmd(a)
	cmd.SetArgs([]string{"docs/a.txt", "docs/b.txt"})
	if err := cmd.ExecuteContext(context.Background()); err != nil {
		t.Fatalf("move command failed: %v", err)
	}

	if gotINM != "*" {
		t.Errorf("If-None-Match = %q, want * (no-overwrite is the default)", gotINM)
	}
}

// TestFilesUploadDefaultSetsIfNoneMatch verifies that without --overwrite,
// upload refuses to clobber by sending If-None-Match: *. --force is unrelated:
// it only bypasses the unchanged-file skip check.
func TestFilesUploadDefaultSetsIfNoneMatch(t *testing.T) {
	var gotINM string
	var gotCT string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotINM = r.Header.Get("If-None-Match")
		gotCT = r.Header.Get("Content-Type")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{}`))
	}))
	t.Cleanup(srv.Close)

	tmp := t.TempDir()
	src := tmp + "/hello.txt"
	if err := writeFileForTest(src, "data"); err != nil {
		t.Fatal(err)
	}

	a, _, _ := newTestApp(t)
	a.API = api.NewClient(srv.URL, "test-key", api.Options{})

	cmd := newFilesUploadCmd(a)
	cmd.SetArgs([]string{"remote.txt", src, "--force"})
	if err := cmd.ExecuteContext(context.Background()); err != nil {
		t.Fatalf("upload command failed: %v", err)
	}

	if gotINM != "*" {
		t.Errorf("If-None-Match = %q, want * (no-overwrite is the default)", gotINM)
	}
	if !strings.HasPrefix(gotCT, "application/octet-stream") {
		t.Errorf("Content-Type = %q, want application/octet-stream", gotCT)
	}
}

// TestFilesUploadOverwriteClearsIfNoneMatch verifies that --overwrite drops
// the If-None-Match: * guard so the upload replaces an existing file.
func TestFilesUploadOverwriteClearsIfNoneMatch(t *testing.T) {
	var gotINM string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotINM = r.Header.Get("If-None-Match")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{}`))
	}))
	t.Cleanup(srv.Close)

	tmp := t.TempDir()
	src := tmp + "/hello.txt"
	if err := writeFileForTest(src, "data"); err != nil {
		t.Fatal(err)
	}

	a, _, _ := newTestApp(t)
	a.API = api.NewClient(srv.URL, "test-key", api.Options{})

	cmd := newFilesUploadCmd(a)
	cmd.SetArgs([]string{"remote.txt", src, "--overwrite", "--force"})
	if err := cmd.ExecuteContext(context.Background()); err != nil {
		t.Fatalf("upload command failed: %v", err)
	}

	if gotINM != "" {
		t.Errorf("If-None-Match = %q, want empty with --overwrite", gotINM)
	}
}

func writeFileForTest(path, content string) error {
	return os.WriteFile(path, []byte(content), 0o644)
}

func TestDownloadSingleFilePreservesDestinationOnStreamFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "64")
		_, _ = w.Write([]byte("partial"))
	}))
	t.Cleanup(server.Close)

	destination := filepath.Join(t.TempDir(), "existing.txt")
	if err := os.WriteFile(destination, []byte("original"), 0o644); err != nil {
		t.Fatal(err)
	}
	client := api.NewClient(server.URL, "test-key", api.Options{})
	if err := downloadSingleFile(context.Background(), client, "proj-1", "remote.txt", destination); err == nil {
		t.Fatal("expected interrupted download to fail")
	}
	got, err := os.ReadFile(destination)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "original" {
		t.Fatalf("destination = %q, want original content", got)
	}
}

func TestDownloadSingleFileCreatesParentsAndAppliesLastModified(t *testing.T) {
	modified := time.Date(2026, 7, 20, 10, 30, 0, 0, time.UTC)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Last-Modified", modified.Format(http.TimeFormat))
		_, _ = w.Write([]byte("complete"))
	}))
	t.Cleanup(server.Close)

	destination := filepath.Join(t.TempDir(), "new", "nested", "file.txt")
	client := api.NewClient(server.URL, "test-key", api.Options{})
	if err := downloadSingleFile(context.Background(), client, "proj-1", "remote.txt", destination); err != nil {
		t.Fatalf("downloadSingleFile: %v", err)
	}
	got, err := os.ReadFile(destination)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "complete" {
		t.Fatalf("destination = %q, want complete", got)
	}
	info, err := os.Stat(destination)
	if err != nil {
		t.Fatal(err)
	}
	if !info.ModTime().Equal(modified) {
		t.Fatalf("mtime = %v, want %v", info.ModTime(), modified)
	}
}
