package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"

	"github.com/agilercloud/cli/internal/publicapi"
)

// WPExecuteResult is the typed result of submitting a wp-cli command.
// Pending=true means the server returned 202; the caller is expected to
// poll GetWPCommand until the status leaves "pending".
type WPExecuteResult struct {
	Command WPCommand
	Pending bool
}

// RunWPCommand submits a wp-cli command against a project. If async is
// true the request carries `Prefer: respond-async`, and the server may
// return 202 with pending metadata.
//
// The WP endpoints type their response as a freeform JSON object in
// the public spec, so the bytes are re-decoded into the typed
// WPCommand here.
func (c *Client) RunWPCommand(ctx context.Context, projectID string, in CreateWPCommand, async bool) (*WPExecuteResult, error) {
	params := &publicapi.CreateWPCommandParams{IdempotencyKey: idempotencyKey()}
	data, err := json.Marshal(in)
	if err != nil {
		return nil, err
	}
	editors := []publicapi.RequestEditorFn{}
	if async {
		editors = append(editors, withHeaders(map[string]string{"Prefer": "respond-async"}))
	}
	resp, err := c.impl.CreateWPCommandWithBodyWithResponse(
		ctx, projectID, params, "application/json", bytes.NewReader(data), editors...,
	)
	if err != nil {
		return nil, err
	}
	if err := checkStatus(resp.StatusCode(), resp.Body); err != nil {
		return nil, err
	}
	var w WPCommand
	if err := json.Unmarshal(resp.Body, &w); err != nil {
		return nil, err
	}
	return &WPExecuteResult{Command: w, Pending: resp.StatusCode() == http.StatusAccepted}, nil
}

// maxWPListPageSize is the server's maximum `limit` for the wp commands
// list endpoint; larger caller limits are fetched in pages of this size
// (the server rejects bigger values with a 400 rather than clamping).
const maxWPListPageSize = 200

// MaxWPOutputPageSize is the server's maximum `limit` for output lines on
// the single-command GET (the default page is 100 lines).
const MaxWPOutputPageSize = 1000

// ListWPCommands returns recent wp-cli commands for a project, newest
// first. Each item is the slim list projection (id, status, submitted_at,
// duration_ms, command_preview); call GetWPCommand to fetch the full
// entity including output lines. limit caps the total entries returned
// and is paginated across Link rel="next" pages as needed; limit ≤ 0
// returns a single page at the server's default size.
func (c *Client) ListWPCommands(ctx context.Context, projectID string, limit int) ([]WPCommandListItem, error) {
	var all []WPCommandListItem
	seen := map[string]struct{}{}
	var cursor *string
	for {
		params := &publicapi.ListWPCommandsParams{Cursor: cursor}
		if limit > 0 {
			remaining := limit - len(all)
			if remaining <= 0 {
				return all, nil
			}
			pageSize := remaining
			if pageSize > maxWPListPageSize {
				pageSize = maxWPListPageSize
			}
			params.Limit = &pageSize
		}
		resp, err := c.impl.ListWPCommandsWithResponse(ctx, projectID, params)
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

// GetWPCommand returns one wp-cli command by ID with a page of output
// lines. limit caps the returned lines (0 = server default of 100, max
// MaxWPOutputPageSize); cursor resumes from a previous page's NextCursor.
// The result's NextCursor is set when more output remains.
func (c *Client) GetWPCommand(ctx context.Context, projectID, commandID string, limit int, cursor string) (*WPCommand, error) {
	params := &publicapi.GetWPCommandParams{}
	if limit > 0 {
		params.Limit = &limit
	}
	if cursor != "" {
		params.Cursor = &cursor
	}
	resp, err := c.impl.GetWPCommandWithResponse(ctx, projectID, commandID, params)
	if err != nil {
		return nil, err
	}
	if err := checkStatus(resp.StatusCode(), resp.Body); err != nil {
		return nil, err
	}
	var w WPCommand
	if err := json.Unmarshal(resp.Body, &w); err != nil {
		return nil, err
	}
	w.NextCursor = nextCursorFromHeaders(resp.HTTPResponse.Header)
	return &w, nil
}

// DeleteWPCommand removes a command (cancels it if it's pending).
func (c *Client) DeleteWPCommand(ctx context.Context, projectID, commandID string) error {
	resp, err := c.impl.DeleteWPCommandWithResponse(
		ctx, projectID, commandID, &publicapi.DeleteWPCommandParams{},
	)
	if err != nil {
		return err
	}
	return checkStatus(resp.StatusCode(), resp.Body)
}
