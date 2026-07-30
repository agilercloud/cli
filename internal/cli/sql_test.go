package cli

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/agilercloud/cli/internal/api"
)

func TestSQLExecuteSubmitsAsyncAndPollsPendingStatement(t *testing.T) {
	gets := 0
	submittedAsync := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/v1/projects/proj-1/sql/statements":
			submittedAsync = r.Header.Get("Prefer") == "respond-async"
			w.WriteHeader(http.StatusAccepted)
			_, _ = w.Write([]byte(`{"id":"stmt-1","sql":"select 1","read_only":false,"status":"pending","columns":[],"rows":[]}`))
		case r.Method == http.MethodGet && r.URL.Path == "/v1/projects/proj-1/sql/statements/stmt-1":
			gets++
			if got := r.URL.Query().Get("limit"); got != "1000" {
				t.Errorf("poll limit = %q, want 1000", got)
			}
			_, _ = w.Write([]byte(`{"id":"stmt-1","sql":"select 1","read_only":false,"status":"success","columns":["answer"],"rows":[[1]]}`))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)

	a, out, _ := newTestApp(t)
	a.API = api.NewClient(server.URL, "test-key", api.Options{})
	cmd := newSQLExecuteCmd(a)
	cmd.SetArgs([]string{"select 1", "--poll-interval=1h"})
	if err := cmd.ExecuteContext(context.Background()); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if !submittedAsync {
		t.Fatal("execute did not submit with Prefer: respond-async")
	}
	if gets != 1 {
		t.Fatalf("poll GETs = %d, want 1", gets)
	}
	if got := out.String(); !strings.Contains(got, "Status:       success") || !strings.Contains(got, "answer") {
		t.Fatalf("final statement output:\n%s", got)
	}
}

func TestSQLExecuteAlreadyTerminalDoesNotPollOrFailTerminalError(t *testing.T) {
	gets := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodPost {
			_, _ = w.Write([]byte(`{"id":"stmt-1","sql":"bad sql","read_only":false,"status":"error","error":"syntax error","columns":[],"rows":[]}`))
			return
		}
		gets++
		http.NotFound(w, r)
	}))
	t.Cleanup(server.Close)

	a, out, _ := newTestApp(t)
	a.API = api.NewClient(server.URL, "test-key", api.Options{})
	cmd := newSQLExecuteCmd(a)
	cmd.SetArgs([]string{"bad sql"})
	if err := cmd.ExecuteContext(context.Background()); err != nil {
		t.Fatalf("SQL terminal error status should be rendered without a CLI error: %v", err)
	}
	if gets != 0 {
		t.Fatalf("terminal submission triggered %d poll requests", gets)
	}
	if got := out.String(); !strings.Contains(got, "Status:       error") || !strings.Contains(got, "syntax error") {
		t.Fatalf("terminal statement output:\n%s", got)
	}
}
