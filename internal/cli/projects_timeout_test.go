package cli

import (
	"encoding/json"
	"github.com/agilercloud/cli/internal/api"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestProjectsUpdateTimeout(t *testing.T) {
	for _, tc := range []struct {
		name string
		args []string
		want any
	}{
		{"minimum", []string{"--timeout", "1"}, float64(1)},
		{"maximum", []string{"--timeout", "180"}, float64(180)},
		{"omitted", []string{"--name", "changed"}, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var body map[string]any
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != "PATCH" || r.URL.Path != "/v1/projects/demo" {
					t.Errorf("request = %s %s", r.Method, r.URL.Path)
				}
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					t.Error(err)
				}
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"timeout":60}`))
			}))
			defer srv.Close()
			a, out, _ := newTestApp(t)
			a.API = api.NewClient(srv.URL, "test-key", api.Options{})
			cmd := newProjectsUpdateCmd(a)
			cmd.SetArgs(append([]string{"demo"}, tc.args...))
			if err := cmd.Execute(); err != nil {
				t.Fatal(err)
			}
			if body["timeout"] != tc.want {
				t.Fatalf("body = %#v", body)
			}
			if !strings.Contains(out.String(), "60 seconds") {
				t.Fatalf("output = %q", out.String())
			}
		})
	}
	for _, value := range []string{"0", "-1", "181", "1.5"} {
		t.Run("invalid_"+value, func(t *testing.T) {
			a, _, _ := newTestApp(t) // No API client: invalid values must fail before sending.
			cmd := newProjectsUpdateCmd(a)
			cmd.SetArgs([]string{"demo", "--timeout", value})
			if err := cmd.Execute(); err == nil {
				t.Fatal("accepted invalid timeout")
			}
		})
	}
}
