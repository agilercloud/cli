package api

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"

	"github.com/agilercloud/cli/internal/publicapi"
)

// commandResponse is the common buffered portion of the generated response
// wrappers used by SQL and wp-cli operations.
type commandResponse struct {
	status  int
	body    []byte
	headers http.Header
}

// submitCommand owns JSON encoding, Prefer header selection, response
// checking, and pending detection for long-running command submissions.
// The typed callback is the only endpoint-specific portion.
func submitCommand[Input, Result any](in Input, async bool, submit func(io.Reader, ...publicapi.RequestEditorFn) (commandResponse, error)) (*Result, bool, error) {
	data, err := json.Marshal(in)
	if err != nil {
		return nil, false, err
	}
	editors := []publicapi.RequestEditorFn{}
	if async {
		editors = append(editors, withHeaders(map[string]string{"Prefer": "respond-async"}))
	}
	resp, err := submit(bytes.NewReader(data), editors...)
	if err != nil {
		return nil, false, err
	}
	value, err := decodeChecked[Result](resp.status, resp.body)
	if err != nil {
		return nil, false, err
	}
	return value, resp.status == http.StatusAccepted, nil
}

// paginateCommandHistory applies the shared limited-history pagination and
// response checks while generated endpoint callbacks provide typed items.
func paginateCommandHistory[T any](limit int, fetch func(cursor *string, pageSize *int) (commandResponse, *[]T, error)) ([]T, error) {
	return paginateLimited(limit, func(cursor *string, pageSize *int) ([]T, http.Header, error) {
		resp, items, err := fetch(cursor, pageSize)
		if err != nil {
			return nil, nil, err
		}
		if err := checkStatus(resp.status, resp.body); err != nil {
			return nil, nil, err
		}
		if items == nil {
			return nil, resp.headers, nil
		}
		return *items, resp.headers, nil
	})
}

// getCommandResult builds optional limit/cursor values, decodes a typed
// command result, and attaches the next-page cursor through a typed callback.
func getCommandResult[T any](limit int, cursor string, fetch func(limit *int, cursor *string) (commandResponse, error), setNextCursor func(*T, string)) (*T, error) {
	var limitParam *int
	if limit > 0 {
		limitParam = &limit
	}
	var cursorParam *string
	if cursor != "" {
		cursorParam = &cursor
	}

	resp, err := fetch(limitParam, cursorParam)
	if err != nil {
		return nil, err
	}
	value, err := decodeChecked[T](resp.status, resp.body)
	if err != nil {
		return nil, err
	}
	setNextCursor(value, nextCursorFromHeaders(resp.headers))
	return value, nil
}

// deleteCommand checks the common buffered response returned by SQL and
// wp-cli delete endpoints.
func deleteCommand(remove func() (commandResponse, error)) error {
	resp, err := remove()
	if err != nil {
		return err
	}
	return checkStatus(resp.status, resp.body)
}
