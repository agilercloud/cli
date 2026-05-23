package cli

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// failingReader emits some bytes, then errors. Simulates a mid-stream
// network drop after the response headers have already been received.
type failingReader struct {
	data []byte
	pos  int
	err  error
}

func (r *failingReader) Read(p []byte) (int, error) {
	if r.pos >= len(r.data) {
		return 0, r.err
	}
	n := copy(p, r.data[r.pos:])
	r.pos += n
	if r.pos >= len(r.data) {
		return n, r.err
	}
	return n, nil
}

// TestWriteStreamAtomicLeavesExistingFileOnFailure verifies that an
// interrupted download does not damage the user's existing file.
func TestWriteStreamAtomicLeavesExistingFileOnFailure(t *testing.T) {
	dir := t.TempDir()
	dst := filepath.Join(dir, "out.bin")

	if err := os.WriteFile(dst, []byte("original-content"), 0o644); err != nil {
		t.Fatal(err)
	}

	boom := errors.New("network reset")
	src := &failingReader{data: []byte("partial-"), err: boom}

	if _, err := writeStreamAtomic(dst, src); err == nil {
		t.Fatal("expected error from failing reader, got nil")
	}

	got, err := os.ReadFile(dst)
	if err != nil {
		t.Fatalf("read destination: %v", err)
	}
	if string(got) != "original-content" {
		t.Errorf("destination corrupted: got %q, want %q", string(got), "original-content")
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read dir: %v", err)
	}
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".agiler-download-") {
			t.Errorf("temp file left behind: %s", e.Name())
		}
	}
}

// TestWriteStreamAtomicSuccessReplacesFile verifies the happy path:
// existing file is replaced by the new content after the copy succeeds.
func TestWriteStreamAtomicSuccessReplacesFile(t *testing.T) {
	dir := t.TempDir()
	dst := filepath.Join(dir, "out.bin")

	if err := os.WriteFile(dst, []byte("original"), 0o644); err != nil {
		t.Fatal(err)
	}

	n, err := writeStreamAtomic(dst, strings.NewReader("new-content"))
	if err != nil {
		t.Fatalf("writeStreamAtomic: %v", err)
	}
	if n != int64(len("new-content")) {
		t.Errorf("byte count = %d, want %d", n, len("new-content"))
	}

	got, err := os.ReadFile(dst)
	if err != nil {
		t.Fatalf("read destination: %v", err)
	}
	if string(got) != "new-content" {
		t.Errorf("destination = %q, want %q", string(got), "new-content")
	}
}

// TestWriteStreamAtomicCreatesNewFile verifies the helper works when
// there is no pre-existing file at outputPath.
func TestWriteStreamAtomicCreatesNewFile(t *testing.T) {
	dir := t.TempDir()
	dst := filepath.Join(dir, "out.bin")

	if _, err := writeStreamAtomic(dst, strings.NewReader("hello")); err != nil {
		t.Fatalf("writeStreamAtomic: %v", err)
	}

	got, err := os.ReadFile(dst)
	if err != nil {
		t.Fatalf("read destination: %v", err)
	}
	if string(got) != "hello" {
		t.Errorf("destination = %q, want %q", string(got), "hello")
	}
}

// Compile-time check that failingReader satisfies io.Reader.
var _ io.Reader = (*failingReader)(nil)
