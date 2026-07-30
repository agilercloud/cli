package api

import (
	"context"
	"io"

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
	statement, pending, err := submitCommand[CreateSQLStatement, SQLStatement](in, async, func(body io.Reader, editors ...publicapi.RequestEditorFn) (commandResponse, error) {
		resp, err := c.impl.RunSQLStatementWithBodyWithResponse(ctx, projectID, params, "application/json", body, editors...)
		if err != nil {
			return commandResponse{}, err
		}
		return commandResponse{status: resp.StatusCode(), body: resp.Body, headers: resp.HTTPResponse.Header}, nil
	})
	if err != nil {
		return nil, err
	}
	return &SQLExecuteResult{Statement: *statement, Pending: pending}, nil
}

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
	return paginateCommandHistory(limit, func(cursor *string, pageSize *int) (commandResponse, *[]SQLStatementListItem, error) {
		resp, err := c.impl.ListProjectSQLStatementsWithResponse(ctx, projectID, &publicapi.ListProjectSQLStatementsParams{Cursor: cursor, Limit: pageSize})
		if err != nil {
			return commandResponse{}, nil, err
		}
		return commandResponse{status: resp.StatusCode(), body: resp.Body, headers: resp.HTTPResponse.Header}, resp.JSON200, nil
	})
}

// GetSQLStatement returns one statement by ID with a page of result rows.
// limit caps the returned rows (0 = server default of 100, max
// MaxSQLRowsPageSize); cursor resumes from a previous page's NextCursor.
// The result's NextCursor is set when more rows remain.
func (c *Client) GetSQLStatement(ctx context.Context, projectID, statementID string, limit int, cursor string) (*SQLStatement, error) {
	return getCommandResult(limit, cursor, func(limit *int, cursor *string) (commandResponse, error) {
		resp, err := c.impl.GetProjectSQLStatementWithResponse(ctx, projectID, statementID, &publicapi.GetProjectSQLStatementParams{Limit: limit, Cursor: cursor})
		if err != nil {
			return commandResponse{}, err
		}
		return commandResponse{status: resp.StatusCode(), body: resp.Body, headers: resp.HTTPResponse.Header}, nil
	}, func(statement *SQLStatement, next string) {
		statement.NextCursor = next
	})
}

// DeleteSQLStatement removes a statement (cancels SQL if it's pending).
func (c *Client) DeleteSQLStatement(ctx context.Context, projectID, statementID string) error {
	return deleteCommand(func() (commandResponse, error) {
		resp, err := c.impl.DeleteProjectSQLStatementWithResponse(ctx, projectID, statementID, &publicapi.DeleteProjectSQLStatementParams{})
		if err != nil {
			return commandResponse{}, err
		}
		return commandResponse{status: resp.StatusCode(), body: resp.Body}, nil
	})
}
