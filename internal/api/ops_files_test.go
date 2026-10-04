package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestFilePathsRoundTrip(t *testing.T) {
	for _, name := range []string{"ai-logic%20(9).php", "docs/read me.txt", "a%2Fb/%zz?# +&.txt", " café/雪.txt "} {
		t.Run(name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if got := r.URL.Path; got != "/v1/projects/test/files/"+name {
					t.Errorf("path = %q, want filename %q", got, name)
				}
				if r.URL.RawQuery != "" {
					t.Errorf("unexpected query: %s", r.URL.RawQuery)
				}
				if r.Method == http.MethodDelete {
					w.WriteHeader(204)
					return
				}
				w.Header().Set("Content-Type", "application/json")
				if r.Method == http.MethodPut {
					_, _ = w.Write([]byte(`{}`))
					return
				}
				_, _ = w.Write([]byte(`[]`))
			}))
			defer server.Close()
			client := NewClient(server.URL, "", Options{})
			ctx := context.Background()
			if _, err := client.ListProjectFiles(ctx, "test", name); err != nil {
				t.Fatal(err)
			}
			resp, err := client.GetProjectFile(ctx, "test", name)
			if err != nil {
				t.Fatal(err)
			}
			resp.Body.Close()
			if err := client.PutProjectFile(ctx, "test", name, "text/plain", strings.NewReader("body"), nil); err != nil {
				t.Fatal(err)
			}
			if err := client.DeleteProjectFile(ctx, "test", name); err != nil {
				t.Fatal(err)
			}
		})
	}
}
