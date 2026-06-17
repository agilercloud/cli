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

// maxSQLListPageSize is the server's maximum `limit` for the statement list
// endpoint; larger caller limits are fetched in pages of this size (the
// server rejects bigger values with a 400 rather than clamping).
const maxSQLListPageSize = 200

// MaxSQLRowsPageSize is the server's maximum `limit` for result rows on the
// single-statement GET (the default page is 100 rows).
const MaxSQLRowsPageSize = 1000

// ListSQLStatements returns recent statements for a project, newest
// first. Each item is the slim list projection (id, status, submitted_at,
// duration_ms, sql_preview); call GetSQLStatement to fetch the full
// entity including rows. limit caps the total entries returned and is
// paginated across Link rel="next" pages as needed; limit ≤ 0 returns a
// single page at the server's default size.
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
			if pageSize > maxSQLListPageSize {
				pageSize = maxSQLListPageSize
			}
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
		if limit <= 0 {
			// No explicit cap: stop at the server's default page rather
			// than crawling the entire history.
			return all, nil
		}
		if len(all) >= limit {
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

// GetSQLStatement returns one statement by ID with a page of result rows.
// limit caps the returned rows (0 = server default of 100, max
// MaxSQLRowsPageSize); cursor resumes from a previous page's NextCursor.
// The result's NextCursor is set when more rows remain.
func (c *Client) GetSQLStatement(ctx context.Context, projectID, statementID string, limit int, cursor string) (*SQLStatement, error) {
	params := &publicapi.GetProjectSQLStatementParams{}
	if limit > 0 {
		params.Limit = &limit
	}
	if cursor != "" {
		params.Cursor = &cursor
	}
	resp, err := c.impl.GetProjectSQLStatementWithResponse(ctx, projectID, statementID, params)
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
	s.NextCursor = nextCursorFromHeaders(resp.HTTPResponse.Header)
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
