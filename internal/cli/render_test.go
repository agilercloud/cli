package cli

import (
	"bytes"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/agilercloud/cli/internal/api"
	"github.com/agilercloud/cli/internal/output"
	"github.com/google/uuid"
)

var update = flag.Bool("update", false, "update golden files")

func assertRender(t *testing.T, name string, render func(*output.Writer), format output.Format, quiet bool) {
	t.Helper()
	var buf bytes.Buffer
	w := output.New(format, quiet, &buf, &bytes.Buffer{})
	render(w)
	goldPath := filepath.Join("testdata", name)
	if *update {
		if err := os.MkdirAll("testdata", 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(goldPath, buf.Bytes(), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(goldPath)
	if err != nil {
		t.Fatalf("missing golden %s: %v (run with -update)", name, err)
	}
	if got := buf.String(); got != string(want) {
		t.Errorf("%s:\n--- want ---\n%s\n--- got ---\n%s", name, want, got)
	}
}

// Helpers for building test fixtures with the wire-shape types.
var (
	testTime = time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	testID1  = uuid.MustParse("00000000-0000-0000-0000-000000000001")
	testID2  = uuid.MustParse("00000000-0000-0000-0000-000000000002")
)

func TestRenderProjectsList(t *testing.T) {
	data := []api.Project{
		{Id: testID1, Name: "alpha", Status: "running", Region: "eu", Runtime: "node22", CreatedAt: testTime, UpdatedAt: testTime},
		{Id: testID2, Name: "beta-service", Status: "stopped", Region: "us", Runtime: "python312", CreatedAt: testTime, UpdatedAt: testTime},
	}
	assertRender(t, "projects_list_text.txt",
		func(w *output.Writer) { renderProjectsList(w, data) }, output.FormatText, false)
	assertRender(t, "projects_list_quiet.txt",
		func(w *output.Writer) { renderProjectsList(w, data) }, output.FormatText, true)
	assertRender(t, "projects_list_empty.txt",
		func(w *output.Writer) { renderProjectsList(w, nil) }, output.FormatText, false)
	assertRender(t, "projects_list_empty_quiet.txt",
		func(w *output.Writer) { renderProjectsList(w, nil) }, output.FormatText, true)
}

func TestRenderProjectDetail(t *testing.T) {
	data := api.ProjectDetail{
		Id:        testID1,
		Name:      "alpha",
		Status:    "running",
		Active:    true,
		Region:    "eu",
		Runtime:   "node22",
		CreatedAt: testTime,
		UpdatedAt: time.Date(2025, 2, 1, 0, 0, 0, 0, time.UTC),
	}
	assertRender(t, "project_detail_text.txt",
		func(w *output.Writer) { _ = renderProjectDetail(w, data) }, output.FormatText, false)
	assertRender(t, "project_detail_quiet.txt",
		func(w *output.Writer) { _ = renderProjectDetail(w, data) }, output.FormatText, true)
}

func TestRenderProjectDetailTabularError(t *testing.T) {
	data := api.ProjectDetail{Id: testID1, Name: "alpha"}
	for _, f := range []output.Format{output.FormatCSV, output.FormatTSV} {
		w := output.New(f, false, &bytes.Buffer{}, &bytes.Buffer{})
		err := renderProjectDetail(w, data)
		if err == nil {
			t.Errorf("renderProjectDetail with format=%s: expected error, got nil", f)
			continue
		}
		if !strings.Contains(err.Error(), string(f)) {
			t.Errorf("error %q should mention format=%s", err, f)
		}
	}
}

func TestRenderRegionsList(t *testing.T) {
	data := []api.Region{
		{Id: "eu-west", Description: "Western Europe"},
		{Id: "us-east", Description: "Eastern US"},
	}
	assertRender(t, "regions_list_text.txt",
		func(w *output.Writer) { renderRegionsList(w, data) }, output.FormatText, false)
	assertRender(t, "regions_list_empty.txt",
		func(w *output.Writer) { renderRegionsList(w, nil) }, output.FormatText, false)
}

func TestRenderRuntimesList(t *testing.T) {
	deprecated := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	data := []api.Runtime{
		{Id: "node22", Description: "Node 22"},
		{Id: "node18", Description: "Node 18", DeprecatedAt: &deprecated},
	}
	assertRender(t, "runtimes_list_text.txt",
		func(w *output.Writer) { renderRuntimesList(w, data) }, output.FormatText, false)
}

func TestRenderFilesList(t *testing.T) {
	data := []api.File{
		{Name: "index.js", Path: "/index.js", Size: 1234, ModifiedAt: testTime, IsDir: false},
		{Name: "src", Path: "/src", Size: 0, ModifiedAt: testTime, IsDir: true},
	}
	assertRender(t, "files_list_text.txt",
		func(w *output.Writer) { renderFilesList(w, data) }, output.FormatText, false)
	assertRender(t, "files_list_quiet.txt",
		func(w *output.Writer) { renderFilesList(w, data) }, output.FormatText, true)
}

func TestRenderVariablesList(t *testing.T) {
	v := "3000"
	data := []api.Variable{
		{Id: testID1, Name: "DATABASE_URL", Sensitive: true, Value: nil},
		{Id: testID2, Name: "PORT", Sensitive: false, Value: &v},
	}
	assertRender(t, "variables_list_text.txt",
		func(w *output.Writer) { renderVariablesList(w, data) }, output.FormatText, false)
}

func TestRenderDomainsList(t *testing.T) {
	data := []api.Domain{
		{Id: testID1, Name: "example.com"},
		{Id: testID2, Name: "api.example.com"},
	}
	assertRender(t, "domains_list_text.txt",
		func(w *output.Writer) { renderDomainsList(w, data) }, output.FormatText, false)
	assertRender(t, "domains_list_empty.txt",
		func(w *output.Writer) { renderDomainsList(w, nil) }, output.FormatText, false)
}

func TestRenderBackupsList(t *testing.T) {
	size := 123
	data := []api.Backup{
		{Id: testID1, Status: "done", CreatedAt: testTime, Automatic: true, Size: &size},
	}
	assertRender(t, "backups_list_text.txt",
		func(w *output.Writer) { renderBackupsList(w, data) }, output.FormatText, false)
}

func TestRenderLogsList(t *testing.T) {
	data := []api.LogEntry{
		{Timestamp: testTime, Priority: "INFO", Message: "hello", RequestId: testID1},
	}
	assertRender(t, "logs_list_text.txt",
		func(w *output.Writer) { renderLogsList(w, data) }, output.FormatText, false)
	assertRender(t, "logs_list_empty.txt",
		func(w *output.Writer) { renderLogsList(w, nil) }, output.FormatText, false)
}

func TestRenderUsageList(t *testing.T) {
	data := []api.Usage{
		{EventsAt: testTime, RequestsTotal: 100, Responses2xx: 90, Responses4xx: 5, Responses5xx: 5, DurationAverage: 12, DatatransferOut: 1},
	}
	assertRender(t, "usage_list_text.txt",
		func(w *output.Writer) { renderUsageList(w, data) }, output.FormatText, false)
}
