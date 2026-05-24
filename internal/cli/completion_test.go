package cli

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/agilercloud/cli/internal/api"
	"github.com/agilercloud/cli/internal/app"
	"github.com/agilercloud/cli/internal/config"
	"github.com/spf13/cobra"
)

func newCompletionApp(t *testing.T, server *httptest.Server) *app.App {
	t.Helper()
	a := &app.App{
		Config: &config.Config{},
		API:    api.NewClient(server.URL, "test-key", api.Options{}),
	}
	return a
}

func runCompletion(t *testing.T, fn cobra.CompletionFunc) ([]string, cobra.ShellCompDirective) {
	t.Helper()
	cmd := &cobra.Command{Use: "test"}
	cmd.SetContext(context.Background())
	return fn(cmd, nil, "")
}

func TestCompleteProjectIDs(t *testing.T) {
	var gotPath, gotQuery string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotQuery = r.URL.RawQuery
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[
			{"id":"00000000-0000-0000-0000-000000000001","name":"alpha","status":"running","active":true,"region":"eu","runtime":"node22","workspace_id":"00000000-0000-0000-0000-000000000010","created_at":"2025-01-01T00:00:00Z","updated_at":"2025-01-01T00:00:00Z"},
			{"id":"00000000-0000-0000-0000-000000000002","name":"beta","status":"stopped","active":false,"region":"us","runtime":"node18","workspace_id":"00000000-0000-0000-0000-000000000010","created_at":"2025-01-01T00:00:00Z","updated_at":"2025-01-01T00:00:00Z"}
		]`))
	}))
	defer server.Close()

	a := newCompletionApp(t, server)
	got, directive := runCompletion(t, completeProjectIDs(a))

	if directive != cobra.ShellCompDirectiveNoFileComp {
		t.Errorf("directive = %v, want NoFileComp", directive)
	}
	want := []string{
		"00000000-0000-0000-0000-000000000001\talpha",
		"00000000-0000-0000-0000-000000000002\tbeta",
	}
	if len(got) != len(want) {
		t.Fatalf("got %#v, want %#v", got, want)
	}
	for i, w := range want {
		if got[i] != w {
			t.Errorf("got[%d] = %q, want %q", i, got[i], w)
		}
	}
	if !strings.HasSuffix(gotPath, "/projects") {
		t.Errorf("unexpected path %q", gotPath)
	}
	if gotQuery != "" {
		t.Errorf("unexpected query %q (no workspace configured)", gotQuery)
	}
}

func TestCompleteProjectIDsScopedToWorkspace(t *testing.T) {
	var gotQuery string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.RawQuery
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[]`))
	}))
	defer server.Close()

	a := newCompletionApp(t, server)
	a.Config.WorkspaceID = "00000000-0000-0000-0000-000000000010"

	_, directive := runCompletion(t, completeProjectIDs(a))
	if directive != cobra.ShellCompDirectiveNoFileComp {
		t.Errorf("directive = %v, want NoFileComp", directive)
	}
	if !strings.Contains(gotQuery, "workspace_id=00000000-0000-0000-0000-000000000010") {
		t.Errorf("query = %q, want workspace_id filter", gotQuery)
	}
}

func TestCompleteProjectIDsAPIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":{"code":"boom","message":"boom"}}`))
	}))
	defer server.Close()

	a := newCompletionApp(t, server)
	got, directive := runCompletion(t, completeProjectIDs(a))
	if directive != cobra.ShellCompDirectiveError {
		t.Errorf("directive = %v, want Error", directive)
	}
	if got != nil {
		t.Errorf("got = %#v, want nil on error", got)
	}
}

func TestCompleteWorkspaceIDs(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[
			{"id":"00000000-0000-0000-0000-000000000010","name":"Main","role":"admin","billing_user_id":"00000000-0000-0000-0000-0000000000aa","require_mfa":false,"created_at":"2025-01-01T00:00:00Z","updated_at":"2025-01-01T00:00:00Z"}
		]`))
	}))
	defer server.Close()

	a := newCompletionApp(t, server)
	got, directive := runCompletion(t, completeWorkspaceIDs(a))
	if directive != cobra.ShellCompDirectiveNoFileComp {
		t.Errorf("directive = %v, want NoFileComp", directive)
	}
	want := []string{"00000000-0000-0000-0000-000000000010\tMain"}
	if len(got) != 1 || got[0] != want[0] {
		t.Errorf("got %#v, want %#v", got, want)
	}
}

func TestCompleteRegionIDs(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[
			{"id":"eu-west","description":"Western Europe","created_at":"2025-01-01T00:00:00Z","updated_at":"2025-01-01T00:00:00Z"},
			{"id":"us-east","description":"Eastern US","created_at":"2025-01-01T00:00:00Z","updated_at":"2025-01-01T00:00:00Z"}
		]`))
	}))
	defer server.Close()

	a := newCompletionApp(t, server)
	got, directive := runCompletion(t, completeRegionIDs(a))
	if directive != cobra.ShellCompDirectiveNoFileComp {
		t.Errorf("directive = %v, want NoFileComp", directive)
	}
	want := []string{
		"eu-west\tWestern Europe",
		"us-east\tEastern US",
	}
	if len(got) != len(want) {
		t.Fatalf("got %#v, want %#v", got, want)
	}
	for i, w := range want {
		if got[i] != w {
			t.Errorf("got[%d] = %q, want %q", i, got[i], w)
		}
	}
}

func TestCompleteRuntimeIDs(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[
			{"id":"node22","description":"Node 22","created_at":"2025-01-01T00:00:00Z","updated_at":"2025-01-01T00:00:00Z"}
		]`))
	}))
	defer server.Close()

	a := newCompletionApp(t, server)
	got, directive := runCompletion(t, completeRuntimeIDs(a))
	if directive != cobra.ShellCompDirectiveNoFileComp {
		t.Errorf("directive = %v, want NoFileComp", directive)
	}
	want := []string{"node22\tNode 22"}
	if len(got) != 1 || got[0] != want[0] {
		t.Errorf("got %#v, want %#v", got, want)
	}
}
