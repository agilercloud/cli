package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func newTestClient(t *testing.T, handler http.HandlerFunc) *Client {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	return NewClient(srv.URL, "test-key")
}

func TestDoJSONSendsAuthAndAccept(t *testing.T) {
	var gotAuth, gotAccept, gotContentType string
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotAccept = r.Header.Get("Accept")
		gotContentType = r.Header.Get("Content-Type")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok": true}`))
	})

	var out map[string]any
	if err := c.DoJSON(context.Background(), "GET", "/x", nil, &out); err != nil {
		t.Fatalf("DoJSON: %v", err)
	}
	if gotAuth != "Bearer test-key" {
		t.Errorf("Authorization: %q", gotAuth)
	}
	if gotAccept != "application/json" {
		t.Errorf("Accept: %q", gotAccept)
	}
	if gotContentType != "" {
		t.Errorf("unexpected Content-Type for GET: %q", gotContentType)
	}
	if out["ok"] != true {
		t.Errorf("body not decoded: %v", out)
	}
}

func TestDoJSONPostSetsContentType(t *testing.T) {
	var gotMethod, gotContentType, gotPath string
	var gotBody []byte
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		gotContentType = r.Header.Get("Content-Type")
		gotBody, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusCreated)
	})

	payload := strings.NewReader(`{"name":"a"}`)
	if err := c.DoJSON(context.Background(), "POST", "/v1/things", payload, nil); err != nil {
		t.Fatalf("DoJSON: %v", err)
	}
	if gotMethod != "POST" || gotPath != "/v1/things" {
		t.Errorf("method/path: %s %s", gotMethod, gotPath)
	}
	if gotContentType != "application/json" {
		t.Errorf("Content-Type: %q", gotContentType)
	}
	if string(gotBody) != `{"name":"a"}` {
		t.Errorf("body: %q", gotBody)
	}
}

func TestDoJSONNilDestDiscards(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"ok":true}`))
	})
	if err := c.DoJSON(context.Background(), "GET", "/x", nil, nil); err != nil {
		t.Fatalf("DoJSON: %v", err)
	}
}

func TestDoJSONIdempotentSetsKey(t *testing.T) {
	var gotKey string
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotKey = r.Header.Get("Idempotency-Key")
		w.WriteHeader(http.StatusAccepted)
	})

	if err := c.DoJSONIdempotent(context.Background(), "POST", "/x", bytes.NewReader([]byte(`{}`)), nil); err != nil {
		t.Fatalf("DoJSONIdempotent: %v", err)
	}
	if len(gotKey) == 0 {
		t.Error("Idempotency-Key not set")
	}
	// Second call should get a different key.
	var second string
	c = newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		second = r.Header.Get("Idempotency-Key")
		w.WriteHeader(http.StatusAccepted)
	})
	if err := c.DoJSONIdempotent(context.Background(), "POST", "/x", nil, nil); err != nil {
		t.Fatalf("DoJSONIdempotent: %v", err)
	}
	if second == gotKey {
		t.Errorf("expected different idempotency keys, got %q twice", gotKey)
	}
}

func TestDoRawCustomHeaders(t *testing.T) {
	var gotCT, gotCustom string
	var gotBody []byte
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotCT = r.Header.Get("Content-Type")
		gotCustom = r.Header.Get("X-Modified-At")
		gotBody, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusOK)
	})

	body := strings.NewReader("binary-data")
	headers := map[string]string{"X-Modified-At": "12345"}
	resp, err := c.DoRaw(context.Background(), "PUT", "/f", "application/octet-stream", headers, body)
	if err != nil {
		t.Fatalf("DoRaw: %v", err)
	}
	resp.Body.Close()

	if gotCT != "application/octet-stream" {
		t.Errorf("Content-Type: %q", gotCT)
	}
	if gotCustom != "12345" {
		t.Errorf("X-Modified-At: %q", gotCustom)
	}
	if string(gotBody) != "binary-data" {
		t.Errorf("body: %q", gotBody)
	}
}

func TestErrorResponses(t *testing.T) {
	cases := []struct {
		status  int
		body    string
		wantMsg string
	}{
		{http.StatusUnauthorized, `{}`, "not authenticated"},
		{http.StatusForbidden, `{}`, "permission denied"},
		{http.StatusNotFound, `{}`, "not found"},
		{http.StatusTooManyRequests, `{}`, "rate limit"},
		{http.StatusInternalServerError, `{}`, "unexpected server error"},
		{http.StatusServiceUnavailable, `{}`, "service unavailable"},
		{http.StatusBadRequest, `{"message":"bad project name"}`, "bad project name"},
	}
	for _, tc := range cases {
		t.Run(http.StatusText(tc.status), func(t *testing.T) {
			c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(tc.status)
				_, _ = w.Write([]byte(tc.body))
			})

			err := c.DoJSON(context.Background(), "GET", "/x", nil, nil)
			if err == nil {
				t.Fatalf("expected error for status %d", tc.status)
			}
			var apiErr *APIError
			if !errors.As(err, &apiErr) {
				t.Fatalf("expected *APIError, got %T", err)
			}
			if apiErr.StatusCode != tc.status {
				t.Errorf("StatusCode: %d, want %d", apiErr.StatusCode, tc.status)
			}
			if !strings.Contains(apiErr.Error(), tc.wantMsg) {
				t.Errorf("Error() = %q, want to contain %q", apiErr.Error(), tc.wantMsg)
			}
		})
	}
}

func TestAPIErrorCustomMessageOverride(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(map[string]string{"message": "project x not found"})
	})

	err := c.DoJSON(context.Background(), "GET", "/x", nil, nil)
	if err == nil {
		t.Fatal("expected error")
	}
	if err.Error() != "project x not found" {
		t.Errorf("Error() = %q, want %q", err.Error(), "project x not found")
	}
}

func TestTrimTrailingSlashBaseURL(t *testing.T) {
	c := NewClient("http://example.com/", "k")
	if c.BaseURL != "http://example.com" {
		t.Errorf("BaseURL not trimmed: %q", c.BaseURL)
	}
}

func TestContextCancellation(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := c.DoJSON(ctx, "GET", "/x", nil, nil)
	if err == nil {
		t.Fatal("expected cancellation error")
	}
}
