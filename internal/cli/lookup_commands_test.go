package cli

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestLookupCommandsUseAPIInitializedDuringExecution(t *testing.T) {
	tests := []struct {
		name   string
		args   []string
		path   string
		body   string
		output string
	}{
		{
			name:   "regions list",
			args:   []string{"regions", "list"},
			path:   "/v1/regions",
			body:   `[{"id":"eu-west","description":"Western Europe","created_at":"2025-01-01T00:00:00Z","updated_at":"2025-01-01T00:00:00Z"}]`,
			output: "eu-west",
		},
		{
			name:   "regions get",
			args:   []string{"regions", "get", "eu-west"},
			path:   "/v1/regions/eu-west",
			body:   `{"id":"eu-west","description":"Western Europe","created_at":"2025-01-01T00:00:00Z","updated_at":"2025-01-01T00:00:00Z"}`,
			output: "eu-west",
		},
		{
			name:   "runtimes list",
			args:   []string{"runtimes", "list"},
			path:   "/v1/runtimes",
			body:   `[{"id":"node22","description":"Node 22","created_at":"2025-01-01T00:00:00Z","updated_at":"2025-01-01T00:00:00Z"}]`,
			output: "node22",
		},
		{
			name:   "runtimes get",
			args:   []string{"runtimes", "get", "node22"},
			path:   "/v1/runtimes/node22",
			body:   `{"id":"node22","description":"Node 22","created_at":"2025-01-01T00:00:00Z","updated_at":"2025-01-01T00:00:00Z"}`,
			output: "node22",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != tt.path {
					t.Errorf("request path = %q, want %q", r.URL.Path, tt.path)
				}
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(tt.body))
			}))
			defer server.Close()

			a, out, errOut := newTestApp(t)
			args := append([]string{"--api-base", server.URL}, tt.args...)
			if code := Run(a, context.Background(), args); code != 0 {
				t.Fatalf("Run() = %d, want 0; stderr: %s", code, errOut.String())
			}
			if !strings.Contains(out.String(), tt.output) {
				t.Errorf("output %q does not contain %q", out.String(), tt.output)
			}
		})
	}
}
