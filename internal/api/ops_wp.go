package api

import (
	"context"
	"io"

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
	command, pending, err := submitCommand[CreateWPCommand, WPCommand](in, async, func(body io.Reader, editors ...publicapi.RequestEditorFn) (commandResponse, error) {
		resp, err := c.impl.CreateWPCommandWithBodyWithResponse(ctx, projectID, params, "application/json", body, editors...)
		if err != nil {
			return commandResponse{}, err
		}
		return commandResponse{status: resp.StatusCode(), body: resp.Body, headers: resp.HTTPResponse.Header}, nil
	})
	if err != nil {
		return nil, err
	}
	return &WPExecuteResult{Command: *command, Pending: pending}, nil
}

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
	return paginateCommandHistory(limit, func(cursor *string, pageSize *int) (commandResponse, *[]WPCommandListItem, error) {
		resp, err := c.impl.ListWPCommandsWithResponse(ctx, projectID, &publicapi.ListWPCommandsParams{Cursor: cursor, Limit: pageSize})
		if err != nil {
			return commandResponse{}, nil, err
		}
		return commandResponse{status: resp.StatusCode(), body: resp.Body, headers: resp.HTTPResponse.Header}, resp.JSON200, nil
	})
}

// GetWPCommand returns one wp-cli command by ID with a page of output
// lines. limit caps the returned lines (0 = server default of 100, max
// MaxWPOutputPageSize); cursor resumes from a previous page's NextCursor.
// The result's NextCursor is set when more output remains.
func (c *Client) GetWPCommand(ctx context.Context, projectID, commandID string, limit int, cursor string) (*WPCommand, error) {
	return getCommandResult(limit, cursor, func(limit *int, cursor *string) (commandResponse, error) {
		resp, err := c.impl.GetWPCommandWithResponse(ctx, projectID, commandID, &publicapi.GetWPCommandParams{Limit: limit, Cursor: cursor})
		if err != nil {
			return commandResponse{}, err
		}
		return commandResponse{status: resp.StatusCode(), body: resp.Body, headers: resp.HTTPResponse.Header}, nil
	}, func(command *WPCommand, next string) {
		command.NextCursor = next
	})
}

// DeleteWPCommand removes a command (cancels it if it's pending).
func (c *Client) DeleteWPCommand(ctx context.Context, projectID, commandID string) error {
	return deleteCommand(func() (commandResponse, error) {
		resp, err := c.impl.DeleteWPCommandWithResponse(ctx, projectID, commandID, &publicapi.DeleteWPCommandParams{})
		if err != nil {
			return commandResponse{}, err
		}
		return commandResponse{status: resp.StatusCode(), body: resp.Body}, nil
	})
}
