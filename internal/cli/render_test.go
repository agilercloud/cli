package cli

import (
	"bytes"
	"flag"
	"os"
	"path/filepath"
	"testing"

	"github.com/agilercloud/cli/internal/api"
	"github.com/agilercloud/cli/internal/output"
)

var update = flag.Bool("update", false, "update golden files")

func assertRender(t *testing.T, name string, render func(*output.Writer), mode output.Mode) {
	t.Helper()
	var buf bytes.Buffer
	w := output.New(mode, &buf, &bytes.Buffer{})
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

func TestRenderProjectsList(t *testing.T) {
	data := []api.Project{
		{ID: "p1", Name: "alpha", Status: "running", Region: "eu", Runtime: "node22"},
		{ID: "p2", Name: "beta-service", Status: "stopped", Region: "us", Runtime: "python312"},
	}
	assertRender(t, "projects_list_text.txt",
		func(w *output.Writer) { renderProjectsList(w, data) }, output.ModeText)
	assertRender(t, "projects_list_quiet.txt",
		func(w *output.Writer) { renderProjectsList(w, data) }, output.ModeQuiet)
	assertRender(t, "projects_list_empty.txt",
		func(w *output.Writer) { renderProjectsList(w, nil) }, output.ModeText)
	assertRender(t, "projects_list_empty_quiet.txt",
		func(w *output.Writer) { renderProjectsList(w, nil) }, output.ModeQuiet)
}

func TestRenderProjectDetail(t *testing.T) {
	data := api.Project{
		ID: "p1", Name: "alpha", Status: "running", Active: true,
		Region: "eu", Runtime: "node22", Instance: 1,
		CreatedAt: "2025-01-01", UpdatedAt: "2025-02-01",
		Domains: []api.Domain{{ID: "d1", Name: "example.com"}},
	}
	assertRender(t, "project_detail_text.txt",
		func(w *output.Writer) { renderProjectDetail(w, data) }, output.ModeText)
	assertRender(t, "project_detail_quiet.txt",
		func(w *output.Writer) { renderProjectDetail(w, data) }, output.ModeQuiet)
}

func TestRenderRegionsList(t *testing.T) {
	data := []api.Region{
		{ID: "eu-west", Description: "Western Europe"},
		{ID: "us-east", Description: "Eastern US"},
	}
	assertRender(t, "regions_list_text.txt",
		func(w *output.Writer) { renderRegionsList(w, data) }, output.ModeText)
	assertRender(t, "regions_list_empty.txt",
		func(w *output.Writer) { renderRegionsList(w, nil) }, output.ModeText)
}

func TestRenderRuntimesList(t *testing.T) {
	deprecated := "2025-01-01"
	data := []api.Runtime{
		{ID: "node22", Description: "Node 22"},
		{ID: "node18", Description: "Node 18", DeprecatedAt: &deprecated},
	}
	assertRender(t, "runtimes_list_text.txt",
		func(w *output.Writer) { renderRuntimesList(w, data) }, output.ModeText)
}

func TestRenderFilesList(t *testing.T) {
	data := []api.File{
		{Name: "index.js", Path: "/index.js", Size: 1234, ModifiedAt: "2025-01-01T00:00:00Z", IsDir: false},
		{Name: "src", Path: "/src", Size: 0, ModifiedAt: "2025-01-01T00:00:00Z", IsDir: true},
	}
	assertRender(t, "files_list_text.txt",
		func(w *output.Writer) { renderFilesList(w, data) }, output.ModeText)
	assertRender(t, "files_list_quiet.txt",
		func(w *output.Writer) { renderFilesList(w, data) }, output.ModeQuiet)
}

func TestRenderVariablesList(t *testing.T) {
	v := "3000"
	data := []api.Variable{
		{ID: "v1", Name: "DATABASE_URL", Secret: true, Value: nil},
		{ID: "v2", Name: "PORT", Secret: false, Value: &v},
	}
	assertRender(t, "variables_list_text.txt",
		func(w *output.Writer) { renderVariablesList(w, data) }, output.ModeText)
}

func TestRenderDomainsList(t *testing.T) {
	data := []api.Domain{
		{ID: "d1", Name: "example.com"},
		{ID: "d2", Name: "api.example.com"},
	}
	assertRender(t, "domains_list_text.txt",
		func(w *output.Writer) { renderDomainsList(w, data) }, output.ModeText)
	assertRender(t, "domains_list_empty.txt",
		func(w *output.Writer) { renderDomainsList(w, nil) }, output.ModeText)
}

func TestRenderBackupsList(t *testing.T) {
	data := api.BackupsResponse{
		Frequency: 24,
		Retention: 7,
		Data: []api.Backup{
			{ID: "b1", Status: "done", CreatedAt: "2025-01-01", Automatic: true, Size: 123},
		},
	}
	assertRender(t, "backups_list_text.txt",
		func(w *output.Writer) { renderBackupsList(w, data) }, output.ModeText)
}

func TestRenderLogsList(t *testing.T) {
	data := []api.LogEntry{
		{Timestamp: "2025-01-01T00:00:00Z", Priority: "INFO", Message: "hello"},
	}
	assertRender(t, "logs_list_text.txt",
		func(w *output.Writer) { renderLogsList(w, data) }, output.ModeText)
	assertRender(t, "logs_list_empty.txt",
		func(w *output.Writer) { renderLogsList(w, nil) }, output.ModeText)
}

func TestRenderUsageList(t *testing.T) {
	data := []api.UsageRecord{
		{EventsAt: "2025-01-01", RequestsTotal: 100, Responses2xx: 90, Responses4xx: 5, Responses5xx: 5, DurationAverage: 12.3, DatatransferOut: 0.5},
	}
	assertRender(t, "usage_list_text.txt",
		func(w *output.Writer) { renderUsageList(w, data) }, output.ModeText)
}
