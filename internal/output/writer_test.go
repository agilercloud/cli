package output

import (
	"bytes"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

var update = flag.Bool("update", false, "update golden files")

func readGolden(t *testing.T, name string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		if *update {
			return ""
		}
		t.Fatalf("missing golden %s: %v (run with -update)", name, err)
	}
	return string(b)
}

func writeGolden(t *testing.T, name, content string) {
	t.Helper()
	if err := os.MkdirAll("testdata", 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join("testdata", name), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func assertGolden(t *testing.T, name, got string) {
	t.Helper()
	if *update {
		writeGolden(t, name, got)
		return
	}
	want := readGolden(t, name)
	if got != want {
		t.Errorf("%s mismatch\n--- want ---\n%s\n--- got ---\n%s", name, want, got)
	}
}

func TestTableText(t *testing.T) {
	var out, errBuf bytes.Buffer
	w := New(ModeText, &out, &errBuf)
	w.Table(
		[]string{"ID", "NAME", "STATUS"},
		[][]string{
			{"p1", "alpha", "running"},
			{"p22", "beta-service", "building"},
			{"p3", "g", "stopped"},
		},
	)
	assertGolden(t, "table_text.txt", out.String())
	if errBuf.Len() != 0 {
		t.Errorf("stderr not empty: %q", errBuf.String())
	}
}

func TestTableJSON(t *testing.T) {
	var out bytes.Buffer
	w := New(ModeJSON, &out, &bytes.Buffer{})
	w.Table(
		[]string{"ID", "NAME"},
		[][]string{{"p1", "alpha"}, {"p2", "beta"}},
	)
	assertGolden(t, "table_json.txt", out.String())
}

func TestTableJSONEmpty(t *testing.T) {
	var out bytes.Buffer
	w := New(ModeJSON, &out, &bytes.Buffer{})
	w.Table([]string{"ID"}, nil)
	assertGolden(t, "table_json_empty.txt", out.String())
}

func TestTableQuiet(t *testing.T) {
	var out bytes.Buffer
	w := New(ModeQuiet, &out, &bytes.Buffer{})
	w.Table(
		[]string{"ID", "NAME"},
		[][]string{{"p1", "alpha"}, {"p2", "beta"}},
	)
	assertGolden(t, "table_quiet.txt", out.String())
}

func TestTableQuietEmpty(t *testing.T) {
	var out bytes.Buffer
	w := New(ModeQuiet, &out, &bytes.Buffer{})
	w.Table([]string{"ID"}, nil)
	if out.Len() != 0 {
		t.Errorf("expected empty output for Quiet empty, got %q", out.String())
	}
}

func TestText(t *testing.T) {
	var out bytes.Buffer
	w := New(ModeText, &out, &bytes.Buffer{})
	w.Text("Hello %s", "world")
	if got := out.String(); got != "Hello world\n" {
		t.Errorf("got %q", got)
	}
}

func TestJSONIndented(t *testing.T) {
	var out bytes.Buffer
	w := New(ModeJSON, &out, &bytes.Buffer{})
	w.JSON(map[string]any{"a": 1, "b": "two"})
	got := out.String()
	if !strings.Contains(got, "  \"a\": 1") {
		t.Errorf("expected 2-space indent, got %q", got)
	}
}

func TestRawJSON(t *testing.T) {
	var out bytes.Buffer
	w := New(ModeText, &out, &bytes.Buffer{})
	w.RawJSON(strings.NewReader(`{"ok":true}`))
	if got := out.String(); got != "{\"ok\":true}\n" {
		t.Errorf("got %q", got)
	}
}

func TestStderr(t *testing.T) {
	var out, errBuf bytes.Buffer
	w := New(ModeText, &out, &errBuf)
	w.Stderr("warn: %d", 42)
	if out.Len() != 0 {
		t.Errorf("stdout should be empty: %q", out.String())
	}
	if got := errBuf.String(); got != "warn: 42\n" {
		t.Errorf("stderr got %q", got)
	}
}

func TestIsAccessors(t *testing.T) {
	cases := []struct {
		mode              Mode
		wantJSON, wantQui bool
	}{
		{ModeText, false, false},
		{ModeJSON, true, false},
		{ModeQuiet, false, true},
	}
	for _, c := range cases {
		w := New(c.mode, nil, nil)
		if w.IsJSON() != c.wantJSON {
			t.Errorf("IsJSON for mode %d = %v, want %v", c.mode, w.IsJSON(), c.wantJSON)
		}
		if w.IsQuiet() != c.wantQui {
			t.Errorf("IsQuiet for mode %d = %v, want %v", c.mode, w.IsQuiet(), c.wantQui)
		}
	}
}
