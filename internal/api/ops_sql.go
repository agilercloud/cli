package api

import (
	"bytes"
	"context"
	"encoding/json"
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
// first. Each item is the slim list projection (id, status, submitted_at,
// duration_ms, sql_preview); call GetSQLStatement to fetch the full
// entity including rows. limit caps the total entries returned and is
// paginated across Link rel="next" pages as needed (limit ≤ 0 returns
// the full history).
func (c *Client) ListSQLStatements(ctx context.Context, projectID string, limit int) ([]SQLStatementListItem, error) {
	var all []SQLStatementListItem
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
		resp, err := c.impl.ListProjectSQLStatementsWithResponse(ctx, projectID, params)
		if err != nil {
			return nil, err
		}
		if err := checkStatus(resp.StatusCode(), resp.Body); err != nil {
			return nil, err
		}
		if resp.JSON200 != nil {
			all = append(all, *resp.JSON200...)
		}
		if limit > 0 && len(all) >= limit {
			return all[:limit], nil
		}
		next := nextCursorFromHeaders(resp.HTTPResponse.Header)
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
