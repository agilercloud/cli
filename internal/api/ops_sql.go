package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"strconv"

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

// ListSQLStatements returns recent statements for a project, newest first.
// limit ≤ 0 leaves the page size at the server default.
func (c *Client) ListSQLStatements(ctx context.Context, projectID string, limit int) ([]SQLStatement, error) {
	var editors []publicapi.RequestEditorFn
	if limit > 0 {
		editors = append(editors, withQueryParam("limit", strconv.Itoa(limit)))
	}
	resp, err := c.impl.ListProjectSQLStatementsWithResponse(ctx, projectID, editors...)
	if err != nil {
		return nil, err
	}
	if err := checkStatus(resp.StatusCode(), resp.Body); err != nil {
		return nil, err
	}
	if len(resp.Body) == 0 {
		return nil, nil
	}
	var list []SQLStatement
	if err := json.Unmarshal(resp.Body, &list); err != nil {
		return nil, err
	}
	return list, nil
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

// withQueryParam appends a query string parameter to the outgoing
// request. Used for endpoints whose query params aren't fully modeled
// on the generated Params struct (e.g. SQL history "limit").
func withQueryParam(key, value string) publicapi.RequestEditorFn {
	return func(_ context.Context, req *http.Request) error {
		q := req.URL.Query()
		q.Set(key, value)
		req.URL.RawQuery = q.Encode()
		return nil
	}
}
