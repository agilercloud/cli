package api

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRequireJSONResponse(t *testing.T) {
	t.Run("returns typed success body", func(t *testing.T) {
		value := struct{ ID string }{ID: "item-1"}
		got, err := requireJSONResponse(http.StatusCreated, nil, &value)
		if err != nil {
			t.Fatalf("requireJSONResponse: %v", err)
		}
		if got != &value {
			t.Fatalf("requireJSONResponse returned %p, want %p", got, &value)
		}
	})

	t.Run("rejects missing success body", func(t *testing.T) {
		_, err := requireJSONResponse[int](http.StatusOK, nil, nil)
		if !errors.Is(err, errEmptyBody) {
			t.Fatalf("requireJSONResponse error = %v, want errEmptyBody", err)
		}
	})

	t.Run("returns API error before checking body", func(t *testing.T) {
		body := []byte(`{"error":{"code":"invalid_request","message":"bad input","field":"name"}}`)
		_, err := requireJSONResponse[int](http.StatusBadRequest, body, nil)
		var apiErr *APIError
		if !errors.As(err, &apiErr) {
			t.Fatalf("requireJSONResponse error = %T %v, want *APIError", err, err)
		}
		if apiErr.StatusCode != http.StatusBadRequest || apiErr.Code != "invalid_request" || apiErr.Message != "bad input" || apiErr.Field != "name" {
			t.Fatalf("APIError = %#v", apiErr)
		}
	})
}

func TestRequireJSONPage(t *testing.T) {
	t.Run("returns items and headers", func(t *testing.T) {
		items := []int{1, 2}
		headers := http.Header{"Link": {`</items?cursor=next>; rel="next"`}}
		got, gotHeaders, err := requireJSONPage(http.StatusOK, nil, &items, headers)
		if err != nil {
			t.Fatalf("requireJSONPage: %v", err)
		}
		if len(got) != 2 || got[0] != 1 || got[1] != 2 {
			t.Fatalf("items = %v, want [1 2]", got)
		}
		if gotHeaders.Get("Link") != headers.Get("Link") {
			t.Fatalf("Link header = %q, want %q", gotHeaders.Get("Link"), headers.Get("Link"))
		}
	})

	t.Run("accepts explicit empty array", func(t *testing.T) {
		items := []int{}
		got, _, err := requireJSONPage(http.StatusOK, nil, &items, nil)
		if err != nil {
			t.Fatalf("requireJSONPage: %v", err)
		}
		if got == nil || len(got) != 0 {
			t.Fatalf("items = %#v, want non-nil empty slice", got)
		}
	})

	t.Run("rejects missing success body", func(t *testing.T) {
		got, gotHeaders, err := requireJSONPage[int](http.StatusOK, nil, nil, http.Header{"Link": {"ignored"}})
		if !errors.Is(err, errEmptyBody) {
			t.Fatalf("requireJSONPage error = %v, want errEmptyBody", err)
		}
		if got != nil || gotHeaders != nil {
			t.Fatalf("requireJSONPage returned items=%v headers=%v on error", got, gotHeaders)
		}
	})

	t.Run("returns API error before checking body", func(t *testing.T) {
		body := []byte(`{"error":{"code":"rate_limited","message":"slow down"}}`)
		got, gotHeaders, err := requireJSONPage[int](http.StatusTooManyRequests, body, nil, http.Header{"Retry-After": {"1"}})
		var apiErr *APIError
		if !errors.As(err, &apiErr) {
			t.Fatalf("requireJSONPage error = %T %v, want *APIError", err, err)
		}
		if apiErr.StatusCode != http.StatusTooManyRequests || apiErr.Code != "rate_limited" {
			t.Fatalf("APIError = %#v", apiErr)
		}
		if got != nil || gotHeaders != nil {
			t.Fatalf("requireJSONPage returned items=%v headers=%v on error", got, gotHeaders)
		}
	})
}

func TestTypedListEndpointsRejectMissingSuccessBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		// Without a JSON content type, the generated client leaves JSON200 nil
		// and lets the response adapter enforce the required-body contract.
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(server.Close)
	client := NewClient(server.URL, "test-key", Options{})

	tests := []struct {
		name string
		list func() error
	}{
		{
			name: "paginated",
			list: func() error {
				_, err := client.ListDomains(context.Background(), "proj")
				return err
			},
		},
		{
			name: "slice valued",
			list: func() error {
				_, err := client.ListWorkspaces(context.Background())
				return err
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.list(); !errors.Is(err, errEmptyBody) {
				t.Fatalf("list error = %v, want errEmptyBody", err)
			}
		})
	}
}
