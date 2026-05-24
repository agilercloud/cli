package api

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"

	"github.com/agilercloud/cli/internal/publicapi"
)

// SQLExecuteResult is the typed result of submitting a SQL statement.
// Pending=true means the server returned 202; the caller is expected to
// poll GetSQLStatement until the status leaves "pending".
type SQLExecuteResult struct {
	Statement SQLStatement
	Pending   bool
}

// RunSQL submits a SQL statement against a project's database. If async
// is true the request carries `Prefer: respond-async`, and the server
// may return 202 with pending metadata.
//
// The SQL endpoints type their response as a freeform JSON object in
// the public spec, so the bytes are re-decoded into the typed
// SQLStatement here.
func (c *Client) RunSQL(ctx context.Context, projectID string, in CreateSQLStatement, async bool) (*SQLExecuteResult, error) {
	params := &publicapi.RunSQLStatementParams{IdempotencyKey: idempotencyKey()}
	data, err := json.Marshal(in)
	if err != nil {
		return nil, err
	}
	editors := []publicapi.RequestEditorFn{}
	if async {
		editors = append(editors, withHeaders(map[string]string{"Prefer": "respond-async"}))
	}
	resp, err := c.impl.RunSQLStatementWithBodyWithResponse(
		ctx, projectID, params, "application/json", bytes.NewReader(data), editors...,
	)
	if err != nil {
		return nil, err
	}
	if err := checkStatus(resp.StatusCode(), resp.Body); err != nil {
		return nil, err
	}
	var s SQLStatement
	if err := json.Unmarshal(resp.Body, &s); err != nil {
		return nil, err
	}
	return &SQLExecuteResult{Statement: s, Pending: resp.StatusCode() == http.StatusAccepted}, nil
}

// ListSQLStatements returns recent statements for a project, newest
// first. limit caps the total entries returned and is paginated across
// Link rel="next" pages as needed (limit ≤ 0 returns the full history).
//
// The /sql/statements response is typed as a freeform JSON object in
// the spec (it's proxied from the edge runtime) but the wire shape is
// actually an array — so we drop down to the raw HTTP client to read
// the body bytes directly and decode into the typed slice.
func (c *Client) ListSQLStatements(ctx context.Context, projectID string, limit int) ([]SQLStatement, error) {
	var all []SQLStatement
	seen := map[string]struct{}{}
	var cursor *string
	for {
		params := &publicapi.ListProjectSQLStatementsParams{Cursor: cursor}
		if limit > 0 {
			remaining := limit - len(all)
			if remaining <= 0 {
				return all, nil
			}
			pageSize := remaining
			params.Limit = &pageSize
		}
		httpResp, err := c.impl.ListProjectSQLStatements(ctx, projectID, params)
		if err != nil {
			return nil, err
		}
		body, readErr := io.ReadAll(httpResp.Body)
		_ = httpResp.Body.Close()
		if readErr != nil {
			return nil, readErr
		}
		if err := checkStatus(httpResp.StatusCode, body); err != nil {
			return nil, err
		}
		if len(body) > 0 {
			var page []SQLStatement
			if err := json.Unmarshal(body, &page); err != nil {
				return nil, err
			}
			all = append(all, page...)
		}
		if limit > 0 && len(all) >= limit {
			return all[:limit], nil
		}
		next := nextCursorFromHeaders(httpResp.Header)
		if next == "" {
			return all, nil
		}
		if _, ok := seen[next]; ok {
			return all, nil
		}
		seen[next] = struct{}{}
		nextCopy := next
		cursor = &nextCopy
	}
}

// GetSQLStatement returns one statement by ID (including any rows page).
func (c *Client) GetSQLStatement(ctx context.Context, projectID, statementID string) (*SQLStatement, error) {
	resp, err := c.impl.GetProjectSQLStatementWithResponse(ctx, projectID, statementID)
	if err != nil {
		return nil, err
	}
	if err := checkStatus(resp.StatusCode(), resp.Body); err != nil {
		return nil, err
	}
	var s SQLStatement
	if err := json.Unmarshal(resp.Body, &s); err != nil {
		return nil, err
	}
	return &s, nil
}

// DeleteSQLStatement removes a statement (cancels SQL if it's pending).
func (c *Client) DeleteSQLStatement(ctx context.Context, projectID, statementID string) error {
	resp, err := c.impl.DeleteProjectSQLStatementWithResponse(
		ctx, projectID, statementID, &publicapi.DeleteProjectSQLStatementParams{},
	)
	if err != nil {
		return err
	}
	return checkStatus(resp.StatusCode(), resp.Body)
}

