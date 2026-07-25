package cli

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/agilercloud/cli/internal/api"
)

func TestWPExecuteReturnsErrorForFailedCommand(t *testing.T) {
	var submittedAsync bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/v1/projects/proj-1/wp/commands":
			submittedAsync = r.Header.Get("Prefer") == "respond-async"
			w.WriteHeader(http.StatusAccepted)
			_, _ = w.Write([]byte(`{"id":"cmd-1","command":"plugin update --all","status":"pending","output":[]}`))
		case r.Method == http.MethodGet && r.URL.Path == "/v1/projects/proj-1/wp/commands/cmd-1":
			_, _ = w.Write([]byte(`{"id":"cmd-1","command":"plugin update --all","status":"error","exit_code":1,"line_count":1,"error":"Error: update failed","output":["Error: update failed"]}`))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)

	a, out, _ := newTestApp(t)
	a.API = api.NewClient(srv.URL, "test-key", api.Options{})
	cmd := newWPExecuteCmd(a)
	cmd.SetArgs([]string{"plugin update --all", "--poll-interval=1ms"})

	err := cmd.ExecuteContext(context.Background())
	if err == nil {
		t.Fatal("execute returned nil error for failed wp-cli command")
	}
	if !strings.Contains(err.Error(), "exit code 1") {
		t.Fatalf("error = %q, want exit code", err)
	}
	if !submittedAsync {
		t.Fatal("execute did not submit with Prefer: respond-async")
	}
	if got := out.String(); !strings.Contains(got, "Status:       error") || !strings.Contains(got, "Error: update failed") {
		t.Fatalf("failed command was not rendered before returning the error:\n%s", got)
	}
}
