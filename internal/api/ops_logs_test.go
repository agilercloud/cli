package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNextCursorFromHeaders(t *testing.T) {
	headers := http.Header{}
	headers.Add("Link", `</v1/projects/proj/logs?limit=10&cursor=abc123>; rel="next"`)

	if got := nextCursorFromHeaders(headers); got != "abc123" {
		t.Fatalf("nextCursorFromHeaders() = %q, want abc123", got)
	}
}

func TestNextCursorFromHeadersIgnoresOtherRels(t *testing.T) {
	headers := http.Header{}
	headers.Add("Link", `</v1/projects/proj/logs>; rel="alternate"`)

	if got := nextCursorFromHeaders(headers); got != "" {
		t.Fatalf("nextCursorFromHeaders() = %q, want empty", got)
	}
}

func TestGetProjectLogsDrainsPages(t *testing.T) {
	var queries []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		queries = append(queries, r.URL.RawQuery)
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Query().Get("cursor") {
		case "":
			w.Header().Set("Link", `</v1/projects/proj/logs?cursor=next-page&limit=1>; rel="next"`)
			_, _ = w.Write([]byte(`[{"request_id":"00000000-0000-0000-0000-000000000001","timestamp":"2025-01-01T00:00:00Z","priority":"INFO","message":"first"}]`))
		case "next-page":
			_, _ = w.Write([]byte(`[{"request_id":"00000000-0000-0000-0000-000000000002","timestamp":"2025-01-01T00:00:01Z","priority":"INFO","message":"second"}]`))
		default:
			t.Fatalf("unexpected cursor %q", r.URL.Query().Get("cursor"))
		}
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-key")
	got, err := client.GetProjectLogs(context.Background(), "proj", LogsQuery{PageSize: 1})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].Message != "first" || got[1].Message != "second" {
		t.Fatalf("logs = %#v", got)
	}
	if len(queries) != 2 {
		t.Fatalf("queries = %#v", queries)
	}
	if queries[0] != "limit=1" {
		t.Fatalf("first query = %q, want limit=1", queries[0])
	}
	if queries[1] != "cursor=next-page&limit=1" && queries[1] != "limit=1&cursor=next-page" {
		t.Fatalf("second query = %q, want cursor and limit", queries[1])
	}
}

func TestGetProjectLogsLimitCapsTotalResults(t *testing.T) {
	var queries []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		queries = append(queries, r.URL.RawQuery)
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Query().Get("cursor") {
		case "":
			w.Header().Set("Link", `</v1/projects/proj/logs?cursor=next-page&limit=1>; rel="next"`)
			_, _ = w.Write([]byte(`[
				{"request_id":"00000000-0000-0000-0000-000000000001","timestamp":"2025-01-01T00:00:00Z","priority":"INFO","message":"first"},
				{"request_id":"00000000-0000-0000-0000-000000000002","timestamp":"2025-01-01T00:00:01Z","priority":"INFO","message":"second"}
			]`))
		case "next-page":
			w.Header().Set("Link", `</v1/projects/proj/logs?cursor=ignored&limit=1>; rel="next"`)
			_, _ = w.Write([]byte(`[
				{"request_id":"00000000-0000-0000-0000-000000000003","timestamp":"2025-01-01T00:00:02Z","priority":"INFO","message":"third"},
				{"request_id":"00000000-0000-0000-0000-000000000004","timestamp":"2025-01-01T00:00:03Z","priority":"INFO","message":"fourth"}
			]`))
		default:
			t.Fatalf("unexpected cursor %q", r.URL.Query().Get("cursor"))
		}
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-key")
	got, err := client.GetProjectLogs(context.Background(), "proj", LogsQuery{Limit: 3})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 || got[0].Message != "first" || got[1].Message != "second" || got[2].Message != "third" {
		t.Fatalf("logs = %#v", got)
	}
	if len(queries) != 2 {
		t.Fatalf("queries = %#v", queries)
	}
	if queries[0] != "limit=3" {
		t.Fatalf("first query = %q, want limit=3", queries[0])
	}
	if queries[1] != "cursor=next-page&limit=1" && queries[1] != "limit=1&cursor=next-page" {
		t.Fatalf("second query = %q, want cursor and remaining limit", queries[1])
	}
}
