package api

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestPaginateAllFollowsCursors(t *testing.T) {
	pages := [][]int{{1, 2}, {3, 4}, {5}}
	calls := 0
	got, err := paginateAll(func(cursor *string) ([]int, http.Header, error) {
		page := pages[calls]
		calls++
		headers := http.Header{}
		if calls < len(pages) {
			// Cursors must be distinct each step; the loop's cycle guard
			// stops on duplicates.
			headers.Set("Link", `</items?cursor=p`+itoa(calls)+`>; rel="next"`)
		}
		_ = cursor
		return page, headers, nil
	})
	if err != nil {
		t.Fatalf("paginateAll: %v", err)
	}
	want := []int{1, 2, 3, 4, 5}
	if len(got) != len(want) {
		t.Fatalf("len = %d, want %d (%v)", len(got), len(want), got)
	}
	for i, v := range want {
		if got[i] != v {
			t.Fatalf("got[%d] = %d, want %d", i, got[i], v)
		}
	}
}

func TestPaginateAllStopsOnDuplicateCursor(t *testing.T) {
	// A server that always returns the same rel="next" cursor would loop
	// forever without the cycle guard; verify we stop on the second sight.
	calls := 0
	got, err := paginateAll(func(cursor *string) ([]int, http.Header, error) {
		calls++
		headers := http.Header{}
		headers.Set("Link", `</items?cursor=same>; rel="next"`)
		return []int{calls}, headers, nil
	})
	if err != nil {
		t.Fatalf("paginateAll: %v", err)
	}
	if calls != 2 {
		t.Fatalf("calls = %d, want 2 (one initial + one with the new cursor, then dedup)", calls)
	}
	if len(got) != 2 {
		t.Fatalf("got = %v, want 2 items", got)
	}
}

func TestPaginateAllSurfacesErrors(t *testing.T) {
	want := errors.New("boom")
	_, err := paginateAll(func(cursor *string) ([]int, http.Header, error) {
		return nil, nil, want
	})
	if !errors.Is(err, want) {
		t.Fatalf("err = %v, want %v", err, want)
	}
}

func TestListDomainsPaginatesAcrossPages(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Query().Get("cursor") {
		case "":
			w.Header().Set("Link", `</v1/projects/proj/domains?cursor=p2>; rel="next"`)
			_, _ = w.Write([]byte(`[{"id":"00000000-0000-0000-0000-000000000001","name":"a.example.com","primary":true,"created_at":"2025-01-01T00:00:00Z"}]`))
		case "p2":
			_, _ = w.Write([]byte(`[{"id":"00000000-0000-0000-0000-000000000002","name":"b.example.com","primary":false,"created_at":"2025-01-01T00:00:00Z"}]`))
		default:
			t.Fatalf("unexpected cursor %q", r.URL.Query().Get("cursor"))
		}
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-key", Options{})
	got, err := client.ListDomains(context.Background(), "proj")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].Name != "a.example.com" || got[1].Name != "b.example.com" {
		t.Fatalf("domains = %#v", got)
	}
}

func TestListSQLStatementsRespectsLimit(t *testing.T) {
	// Pages of one item each. With limit=2, the loop should stop after
	// pulling exactly two items even though the server keeps offering
	// more via Link rel="next".
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Link", `</v1/projects/proj/sql/statements?cursor=more>; rel="next"`)
		_, _ = w.Write([]byte(`[{"id":"00000000-0000-0000-0000-00000000000` + itoa(calls) + `","status":"success","submitted_at":"2025-01-01T00:00:00Z"}]`))
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-key", Options{})
	got, err := client.ListSQLStatements(context.Background(), "proj", 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d items, want 2", len(got))
	}
}

// itoa returns the decimal text of n (n in [1..9] is enough for these
// tests; avoids pulling strconv just for a one-character formatter).
func itoa(n int) string {
	return string(rune('0' + n))
}
