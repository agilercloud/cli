package api

import (
	"context"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/agilercloud/cli/internal/publicapi"
)

// EncodeFilePath URL-encodes each segment of a project-relative file
// path, leaving the slashes between segments intact. Exported so CLI
// code can build file paths and X-Move-Source / X-Copy-Source header
// values that match the canonical /v1/projects/{id}/files/{path} URI
// encoding.
//
// The generated client treats the whole `{path}` parameter as a single
// segment, which would double-escape slashes — so the wrappers below
// hand it a pre-encoded string.
func EncodeFilePath(p string) string {
	p = strings.TrimPrefix(p, "/")
	segs := strings.Split(p, "/")
	for i, s := range segs {
		segs[i] = url.PathEscape(s)
	}
	return strings.Join(segs, "/")
}

// ListProjectFiles returns the contents of a project directory.
// An empty remotePath lists the project root.
func (c *Client) ListProjectFiles(ctx context.Context, projectID, remotePath string) ([]File, error) {
	if remotePath == "" {
		resp, err := c.impl.ListProjectFilesWithResponse(ctx, projectID)
		if err != nil {
			return nil, err
		}
		if err := checkStatus(resp.StatusCode(), resp.Body); err != nil {
			return nil, err
		}
		if resp.JSON200 == nil {
			return nil, errEmptyBody
		}
		return *resp.JSON200, nil
	}
	resp, err := c.impl.GetProjectFileWithResponse(ctx, projectID, EncodeFilePath(remotePath))
	if err != nil {
		return nil, err
	}
	if err := checkStatus(resp.StatusCode(), resp.Body); err != nil {
		return nil, err
	}
	if resp.JSON200 == nil {
		return nil, errEmptyBody
	}
	return *resp.JSON200, nil
}

// GetProjectFile streams the raw bytes of a project file. The caller
// owns closing resp.Body. Non-2xx statuses are decoded into APIError
// and the body is closed before return.
func (c *Client) GetProjectFile(ctx context.Context, projectID, remotePath string) (*http.Response, error) {
	p := "/v1/projects/" + projectID + "/files/" + EncodeFilePath(remotePath)
	resp, err := c.rawGET(ctx, p)
	if err != nil {
		return nil, err
	}
	if err := decodeErrorFromHTTPResponse(resp); err != nil {
		return nil, err
	}
	return resp, nil
}

// PutProjectFile uploads a file's bytes. contentType is the wire
// Content-Type (typically application/octet-stream). Pass "" for the
// body-less move/copy mode — the wrapper strips the Content-Type the
// generated client would otherwise force.
//
// extraHeaders carries per-request headers the spec doesn't model:
// If-None-Match for the no-clobber guard, Last-Modified for client-
// supplied mtime, X-Move-Source / X-Copy-Source for move/copy.
func (c *Client) PutProjectFile(ctx context.Context, projectID, remotePath, contentType string, body io.Reader, extraHeaders map[string]string) error {
	params := &publicapi.PutProjectFileParams{}
	editors := []publicapi.RequestEditorFn{}
	if len(extraHeaders) > 0 {
		editors = append(editors, withHeaders(extraHeaders))
	}
	if contentType == "" {
		// Body-less move/copy mode must not advertise a Content-Type.
		// PutProjectFileWithBody always sets one, so strip it after.
		editors = append(editors, func(_ context.Context, req *http.Request) error {
			req.Header.Del("Content-Type")
			return nil
		})
	}
	resp, err := c.impl.PutProjectFileWithBodyWithResponse(
		ctx, projectID, EncodeFilePath(remotePath), params, contentType, body, editors...,
	)
	if err != nil {
		return err
	}
	return checkStatus(resp.StatusCode(), resp.Body)
}

// DeleteProjectFile removes a file or directory.
func (c *Client) DeleteProjectFile(ctx context.Context, projectID, remotePath string) error {
	resp, err := c.impl.DeleteProjectFileWithResponse(
		ctx, projectID, EncodeFilePath(remotePath), &publicapi.DeleteProjectFileParams{},
	)
	if err != nil {
		return err
	}
	return checkStatus(resp.StatusCode(), resp.Body)
}
