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

var sampleHeaders = []string{"ID", "NAME", "STATUS"}
var sampleRows = [][]string{
	{"p1", "alpha", "running"},
	{"p22", "beta-service", "building"},
	{"p3", "g", "stopped"},
}

func TestTableText(t *testing.T) {
	var out, errBuf bytes.Buffer
	w := New(FormatText, false, &out, &errBuf)
	w.Table(sampleHeaders, sampleRows)
	assertGolden(t, "table_text.txt", out.String())
	if errBuf.Len() != 0 {
		t.Errorf("stderr not empty: %q", errBuf.String())
	}
}

func TestTableJSON(t *testing.T) {
	var out bytes.Buffer
	w := New(FormatJSON, false, &out, &bytes.Buffer{})
	w.Table(
		[]string{"ID", "NAME"},
		[][]string{{"p1", "alpha"}, {"p2", "beta"}},
	)
	assertGolden(t, "table_json.txt", out.String())
}

func TestTableJSONEmpty(t *testing.T) {
	var out bytes.Buffer
	w := New(FormatJSON, false, &out, &bytes.Buffer{})
	w.Table([]string{"ID"}, nil)
	assertGolden(t, "table_json_empty.txt", out.String())
}

func TestTableQuiet(t *testing.T) {
	var out bytes.Buffer
	w := New(FormatText, true, &out, &bytes.Buffer{})
	w.Table(
		[]string{"ID", "NAME"},
		[][]string{{"p1", "alpha"}, {"p2", "beta"}},
	)
	assertGolden(t, "table_quiet.txt", out.String())
}

func TestTableQuietEmpty(t *testing.T) {
	var out bytes.Buffer
	w := New(FormatText, true, &out, &bytes.Buffer{})
	w.Table([]string{"ID"}, nil)
	if out.Len() != 0 {
		t.Errorf("expected empty output for Quiet empty, got %q", out.String())
	}
}

func TestTableYAML(t *testing.T) {
	var out bytes.Buffer
	w := New(FormatYAML, false, &out, &bytes.Buffer{})
	w.Table(
		[]string{"ID", "NAME"},
		[][]string{{"p1", "alpha"}, {"p2", "beta"}},
	)
	assertGolden(t, "table_yaml.txt", out.String())
}

func TestTableYAMLQuiet(t *testing.T) {
	var out bytes.Buffer
	w := New(FormatYAML, true, &out, &bytes.Buffer{})
	w.Table(sampleHeaders, sampleRows)
	assertGolden(t, "table_yaml_quiet.txt", out.String())
}

func TestTableJSONQuiet(t *testing.T) {
	var out bytes.Buffer
	w := New(FormatJSON, true, &out, &bytes.Buffer{})
	w.Table(sampleHeaders, sampleRows)
	assertGolden(t, "table_json_quiet.txt", out.String())
}

func TestTableCSV(t *testing.T) {
	var out bytes.Buffer
	w := New(FormatCSV, false, &out, &bytes.Buffer{})
	w.Table(sampleHeaders, sampleRows)
	assertGolden(t, "table_csv.txt", out.String())
}

func TestTableCSVQuiet(t *testing.T) {
	var out bytes.Buffer
	w := New(FormatCSV, true, &out, &bytes.Buffer{})
	w.Table(sampleHeaders, sampleRows)
	assertGolden(t, "table_csv_quiet.txt", out.String())
}

func TestTableTSV(t *testing.T) {
	var out bytes.Buffer
	w := New(FormatTSV, false, &out, &bytes.Buffer{})
	w.Table(sampleHeaders, sampleRows)
	assertGolden(t, "table_tsv.txt", out.String())
}

func TestText(t *testing.T) {
	var out bytes.Buffer
	w := New(FormatText, false, &out, &bytes.Buffer{})
	w.Text("Hello %s", "world")
	if got := out.String(); got != "Hello world\n" {
		t.Errorf("got %q", got)
	}
}

func TestStructuredJSONIndented(t *testing.T) {
	var out bytes.Buffer
	w := New(FormatJSON, false, &out, &bytes.Buffer{})
	w.Structured(map[string]any{"a": 1, "b": "two"})
	got := out.String()
	if !strings.Contains(got, "  \"a\": 1") {
		t.Errorf("expected 2-space indent, got %q", got)
	}
}

func TestStructuredYAML(t *testing.T) {
	var out bytes.Buffer
	w := New(FormatYAML, false, &out, &bytes.Buffer{})
	w.Structured(map[string]any{"a": 1, "b": "two"})
	got := out.String()
	if !strings.Contains(got, "a: 1") || !strings.Contains(got, "b: two") {
		t.Errorf("expected yaml output, got %q", got)
	}
}

func TestRawJSON(t *testing.T) {
	var out bytes.Buffer
	w := New(FormatText, false, &out, &bytes.Buffer{})
	w.RawJSON(strings.NewReader(`{"ok":true}`))
	if got := out.String(); got != "{\"ok\":true}\n" {
		t.Errorf("got %q", got)
	}
}

func TestStderr(t *testing.T) {
	var out, errBuf bytes.Buffer
	w := New(FormatText, false, &out, &errBuf)
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
		format                Format
		quiet                 bool
		wantStruct, wantTable bool
		wantQui               bool
	}{
		{FormatText, false, false, false, false},
		{FormatJSON, false, true, false, false},
		{FormatYAML, false, true, false, false},
		{FormatCSV, false, false, true, false},
		{FormatTSV, false, false, true, false},
		{FormatText, true, false, false, true},
		{FormatJSON, true, true, false, true},
	}
	for _, c := range cases {
		w := New(c.format, c.quiet, nil, nil)
		if w.IsStructured() != c.wantStruct {
			t.Errorf("IsStructured(%s) = %v, want %v", c.format, w.IsStructured(), c.wantStruct)
		}
		if w.IsTabular() != c.wantTable {
			t.Errorf("IsTabular(%s) = %v, want %v", c.format, w.IsTabular(), c.wantTable)
		}
		if w.IsQuiet() != c.wantQui {
			t.Errorf("IsQuiet(quiet=%v) = %v, want %v", c.quiet, w.IsQuiet(), c.wantQui)
		}
	}
}

func TestParseFormat(t *testing.T) {
	cases := []struct {
		in      string
		want    Format
		wantErr bool
	}{
		{"text", FormatText, false},
		{"json", FormatJSON, false},
		{"yaml", FormatYAML, false},
		{"csv", FormatCSV, false},
		{"tsv", FormatTSV, false},
		{"", "", true},
		{"xml", "", true},
		{"JSON", "", true},
	}
	for _, c := range cases {
		got, err := ParseFormat(c.in)
		if (err != nil) != c.wantErr {
			t.Errorf("ParseFormat(%q) err=%v, wantErr=%v", c.in, err, c.wantErr)
			continue
		}
		if got != c.want {
			t.Errorf("ParseFormat(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}
