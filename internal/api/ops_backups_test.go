package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRestoreBackupDrainRequestsParam(t *testing.T) {
	tests := []struct {
		name          string
		drainRequests bool
		wantQuery     string
	}{
		{name: "default immediate restore omits query", drainRequests: false, wantQuery: ""},
		{name: "drain requests sends query", drainRequests: true, wantQuery: "drain_requests=true"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gotMethod, gotPath, gotQuery string
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotMethod = r.Method
				gotPath = r.URL.Path
				gotQuery = r.URL.RawQuery
				w.WriteHeader(http.StatusAccepted)
			}))
			defer server.Close()

			client := NewClient(server.URL, "test-key")
			if err := client.RestoreBackup(context.Background(), "project_1", "backup_1", tt.drainRequests); err != nil {
				t.Fatalf("RestoreBackup() error = %v", err)
			}

			if gotMethod != http.MethodPost {
				t.Fatalf("method = %s, want %s", gotMethod, http.MethodPost)
			}
			if gotPath != "/v1/projects/project_1/backups/backup_1/restore" {
				t.Fatalf("path = %s", gotPath)
			}
			if gotQuery != tt.wantQuery {
				t.Fatalf("query = %q, want %q", gotQuery, tt.wantQuery)
			}
		})
	}
}
