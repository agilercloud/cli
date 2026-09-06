package cli

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRuleCatalogPreservesTemplateMetadata(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/v1/rules" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"conditions":[],"actions":[],"templates":[{"name":"Maintenance","description":"Set your hostname and operator IP.","requires_configuration":true,"priority":30,"conditions":{},"actions":[]}]}`))
	}))
	defer server.Close()
	a, out, errOut := newTestApp(t)
	if code := Run(a, context.Background(), []string{"--api-base", server.URL, "rules", "templates", "options"}); code != 0 {
		t.Fatalf("exit = %d: %s", code, errOut.String())
	}
	var catalog struct {
		Templates []map[string]any `json:"templates"`
	}
	if err := json.Unmarshal(out.Bytes(), &catalog); err != nil {
		t.Fatal(err)
	}
	if len(catalog.Templates) != 1 {
		t.Fatalf("templates = %#v", catalog.Templates)
	}
	for key, want := range map[string]any{"description": "Set your hostname and operator IP.", "requires_configuration": true, "priority": float64(30)} {
		if got := catalog.Templates[0][key]; got != want {
			t.Errorf("%s = %#v, want %#v", key, got, want)
		}
	}
}
