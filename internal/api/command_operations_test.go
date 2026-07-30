package api

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSQLAndWPCommandOperationsHaveRequestParity(t *testing.T) {
	tests := []struct {
		name           string
		collectionPath string
		itemPath       string
		submitBody     string
		getBody        string
		run            func(*Client) (string, bool, error)
		list           func(*Client) error
		get            func(*Client) (string, error)
		remove         func(*Client) error
	}{
		{
			name:           "sql",
			collectionPath: "/v1/projects/proj/sql/statements",
			itemPath:       "/v1/projects/proj/sql/statements/item-1",
			submitBody:     `{"id":"item-1","sql":"select 1","read_only":false,"status":"pending","columns":[],"rows":[]}`,
			getBody:        `{"id":"item-1","sql":"select 1","read_only":false,"status":"success","columns":[],"rows":[]}`,
			run: func(client *Client) (string, bool, error) {
				result, err := client.RunSQL(context.Background(), "proj", CreateSQLStatement{Sql: "select 1"}, true)
				if err != nil {
					return "", false, err
				}
				return result.Statement.ID, result.Pending, nil
			},
			list: func(client *Client) error {
				_, err := client.ListSQLStatements(context.Background(), "proj", 1)
				return err
			},
			get: func(client *Client) (string, error) {
				result, err := client.GetSQLStatement(context.Background(), "proj", "item-1", 7, "cursor-in")
				if err != nil {
					return "", err
				}
				return result.NextCursor, nil
			},
			remove: func(client *Client) error {
				return client.DeleteSQLStatement(context.Background(), "proj", "item-1")
			},
		},
		{
			name:           "wp",
			collectionPath: "/v1/projects/proj/wp/commands",
			itemPath:       "/v1/projects/proj/wp/commands/item-1",
			submitBody:     `{"id":"item-1","command":"plugin list","status":"pending","output":[]}`,
			getBody:        `{"id":"item-1","command":"plugin list","status":"success","output":[]}`,
			run: func(client *Client) (string, bool, error) {
				result, err := client.RunWPCommand(context.Background(), "proj", CreateWPCommand{Command: "plugin list"}, true)
				if err != nil {
					return "", false, err
				}
				return result.Command.ID, result.Pending, nil
			},
			list: func(client *Client) error {
				_, err := client.ListWPCommands(context.Background(), "proj", 1)
				return err
			},
			get: func(client *Client) (string, error) {
				result, err := client.GetWPCommand(context.Background(), "proj", "item-1", 7, "cursor-in")
				if err != nil {
					return "", err
				}
				return result.NextCursor, nil
			},
			remove: func(client *Client) error {
				return client.DeleteWPCommand(context.Background(), "proj", "item-1")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				switch {
				case r.Method == http.MethodPost && r.URL.Path == tt.collectionPath:
					if got := r.Header.Get("Prefer"); got != "respond-async" {
						t.Errorf("Prefer = %q, want respond-async", got)
					}
					if got := r.Header.Get("Idempotency-Key"); got == "" {
						t.Error("missing Idempotency-Key")
					}
					w.WriteHeader(http.StatusAccepted)
					_, _ = w.Write([]byte(tt.submitBody))
				case r.Method == http.MethodGet && r.URL.Path == tt.collectionPath:
					if got := r.URL.Query().Get("limit"); got != "1" {
						t.Errorf("history limit = %q, want 1", got)
					}
					_, _ = w.Write([]byte(`[]`))
				case r.Method == http.MethodGet && r.URL.Path == tt.itemPath:
					if got := r.URL.Query().Get("limit"); got != "7" {
						t.Errorf("get limit = %q, want 7", got)
					}
					if got := r.URL.Query().Get("cursor"); got != "cursor-in" {
						t.Errorf("get cursor = %q, want cursor-in", got)
					}
					w.Header().Set("Link", `<`+tt.itemPath+`?cursor=cursor-out>; rel="next"`)
					_, _ = w.Write([]byte(tt.getBody))
				case r.Method == http.MethodDelete && r.URL.Path == tt.itemPath:
					w.WriteHeader(http.StatusNoContent)
				default:
					t.Errorf("unexpected request %s %s", r.Method, r.URL.String())
					http.NotFound(w, r)
				}
			}))
			t.Cleanup(server.Close)
			client := NewClient(server.URL, "test-key", Options{})

			id, pending, err := tt.run(client)
			if err != nil {
				t.Fatalf("submit: %v", err)
			}
			if id != "item-1" || !pending {
				t.Fatalf("submit result = id %q, pending %v", id, pending)
			}
			if err := tt.list(client); err != nil {
				t.Fatalf("list: %v", err)
			}
			next, err := tt.get(client)
			if err != nil {
				t.Fatalf("get: %v", err)
			}
			if next != "cursor-out" {
				t.Fatalf("next cursor = %q, want cursor-out", next)
			}
			if err := tt.remove(client); err != nil {
				t.Fatalf("delete: %v", err)
			}
		})
	}
}

func TestCommandOperationsRejectEmptySuccessBodies(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			w.WriteHeader(http.StatusAccepted)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(server.Close)
	client := NewClient(server.URL, "test-key", Options{})

	tests := []struct {
		name   string
		submit func() error
		get    func() error
	}{
		{
			name: "sql",
			submit: func() error {
				_, err := client.RunSQL(context.Background(), "proj", CreateSQLStatement{Sql: "select 1"}, true)
				return err
			},
			get: func() error {
				_, err := client.GetSQLStatement(context.Background(), "proj", "item-1", 0, "")
				return err
			},
		},
		{
			name: "wp",
			submit: func() error {
				_, err := client.RunWPCommand(context.Background(), "proj", CreateWPCommand{Command: "plugin list"}, true)
				return err
			},
			get: func() error {
				_, err := client.GetWPCommand(context.Background(), "proj", "item-1", 0, "")
				return err
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.submit(); err == nil {
				t.Fatal("submit accepted an empty success body")
			}
			if err := tt.get(); err == nil {
				t.Fatal("get accepted an empty success body")
			}
		})
	}
}

func TestCreatedResourceRequiresTypedBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
	}))
	t.Cleanup(server.Close)
	client := NewClient(server.URL, "test-key", Options{})

	_, domainErr := client.CreateDomain(context.Background(), "proj", CreateDomainInput{Name: "example.com"})
	if !errors.Is(domainErr, errEmptyBody) {
		t.Fatalf("CreateDomain error = %v, want errEmptyBody", domainErr)
	}
	_, variableErr := client.CreateVariable(context.Background(), "proj", CreateVariableInput{Name: "KEY", Value: "value"})
	if !errors.Is(variableErr, errEmptyBody) {
		t.Fatalf("CreateVariable error = %v, want errEmptyBody", variableErr)
	}
}
