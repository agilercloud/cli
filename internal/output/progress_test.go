package output

import (
	"bytes"
	"io"
	"strings"
	"testing"
)

// TestProgressReaderRendersAndForwardsBytes feeds 1 MB through a ProgressReader
// and verifies that some renders land on the writer, the byte count matches
// the source, the label appears, and that Finish writes a trailing newline.
func TestProgressReaderRendersAndForwardsBytes(t *testing.T) {
	const size = 1 << 20 // 1 MB
	src := bytes.NewReader(make([]byte, size))
	var w bytes.Buffer

	p := NewProgressReader(src, &w, "test.bin", int64(size), Color{})
	p.minInterval = 0

	n, err := io.Copy(io.Discard, p)
	if err != nil {
		t.Fatalf("copy: %v", err)
	}
	if n != int64(size) {
		t.Errorf("copied %d bytes, want %d", n, size)
	}

	p.Finish(true)

	out := w.String()
	if out == "" {
		t.Fatal("no progress output written")
	}
	if !strings.Contains(out, "test.bin") {
		t.Errorf("output missing label: %q", out)
	}
	if !strings.Contains(out, "\r") {
		t.Errorf("output missing carriage return: %q", out)
	}
	if !strings.HasSuffix(out, "\n") {
		t.Errorf("output should end with newline, got %q", out[max(0, len(out)-16):])
	}
	if !strings.Contains(out, "done") {
		t.Errorf("final frame missing done suffix: %q", out)
	}
}

// TestProgressReaderShowsPercentWhenTotalKnown verifies a percent token
// renders only when Total > 0.
func TestProgressReaderShowsPercentWhenTotalKnown(t *testing.T) {
	const size = 1024
	var w bytes.Buffer
	p := NewProgressReader(bytes.NewReader(make([]byte, size)), &w, "x", int64(size), Color{})
	p.minInterval = 0
	if _, err := io.Copy(io.Discard, p); err != nil {
		t.Fatal(err)
	}
	p.Finish(true)
	if !strings.Contains(w.String(), "%") {
		t.Errorf("expected percent in output, got %q", w.String())
	}
}

// TestProgressReaderOmitsPercentWhenTotalUnknown verifies that Total=0
// suppresses the percent token (Content-Length absent).
func TestProgressReaderOmitsPercentWhenTotalUnknown(t *testing.T) {
	const size = 1024
	var w bytes.Buffer
	p := NewProgressReader(bytes.NewReader(make([]byte, size)), &w, "x", 0, Color{})
	p.minInterval = 0
	if _, err := io.Copy(io.Discard, p); err != nil {
		t.Fatal(err)
	}
	p.Finish(true)
	if strings.Contains(w.String(), "%") {
		t.Errorf("expected no percent when Total=0, got %q", w.String())
	}
}

// TestProgressReaderFinishIdempotent verifies that calling Finish twice
// does not emit a second final line.
func TestProgressReaderFinishIdempotent(t *testing.T) {
	var w bytes.Buffer
	p := NewProgressReader(strings.NewReader("hello"), &w, "f", 5, Color{})
	p.minInterval = 0
	if _, err := io.Copy(io.Discard, p); err != nil {
		t.Fatal(err)
	}
	p.Finish(true)
	first := w.Len()
	p.Finish(true)
	if w.Len() != first {
		t.Errorf("second Finish wrote %d more bytes", w.Len()-first)
	}
}

// TestFormatBytes spot-checks the unit thresholds.
func TestFormatBytes(t *testing.T) {
	cases := []struct {
		n    int64
		want string
	}{
		{0, "0 B"},
		{512, "512 B"},
		{1 << 10, "1.0 KB"},
		{1 << 20, "1.0 MB"},
		{1<<30 + 1<<29, "1.5 GB"},
	}
	for _, c := range cases {
		if got := formatBytes(c.n); got != c.want {
			t.Errorf("formatBytes(%d) = %q, want %q", c.n, got, c.want)
		}
	}
}
