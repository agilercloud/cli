// Package api is the CLI's service-layer wrapper around the generated
// publicapi client. CLI commands call methods on *Client; each method
// hides the generated client's trailing reqEditors parameter, takes
// care of Idempotency-Key generation for unsafe operations, and turns
// non-2xx responses into a typed *APIError.
//
// Body-returning operations return the decoded payload (e.g. a slice of
// ProjectSummary). Streaming operations (file downloads, PDF statements)
// expose the raw *http.Response so the caller can pipe the body without
// buffering.
package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/agilercloud/cli/internal/publicapi"
	"github.com/google/uuid"
)

// Options bundles optional knobs for NewClient. The zero value is valid
// and matches the historical NewClient(baseURL, apiKey) behavior.
type Options struct {
	// Debug, when non-nil, receives a textual log of every HTTP request
	// and response made by the client. Intended for the --debug flag;
	// the Authorization header is redacted before being written.
	Debug io.Writer
}

// Client is the agiler API client. It wraps a generated publicapi
// client (for typed v1 operations) and a plain http.Client (for the
// unversioned /status health endpoint, which is outside the v1 spec).
type Client struct {
	// impl is the typed generated client. Requests it issues are
	// prefixed by base (which already includes /v1).
	impl *publicapi.ClientWithResponses

	// base is the configured API base URL with the /v1 suffix
	// appended — what the generated client uses as its Server.
	base string

	// raw is the underlying http.Client shared with impl, used for
	// direct GETs against /status (not in the public spec).
	raw *http.Client

	// apiKey is retained so direct calls can authenticate consistently
	// with the generated client's request editor.
	apiKey string
}

// NewClient builds a Client for the given API base URL and bearer token.
// baseURL is the unversioned root (e.g. https://api.agiler.io); /v1 is
// appended internally so callers and tests can keep using the historical
// base form.
//
// The HTTP client deliberately omits http.Client.Timeout: that field
// caps the entire request including body read, which would truncate
// large streaming downloads (backups, project files, billing PDFs).
// Connection setup and the wait for response headers are still bounded
// via the transport; once headers arrive, the caller's context.Context
// is the only deadline on body read.
func NewClient(baseURL, apiKey string, opts Options) *Client {
	base := strings.TrimSuffix(baseURL, "/") + "/v1"
	var transport http.RoundTripper = &http.Transport{
		DialContext: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		ResponseHeaderTimeout: 30 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		MaxIdleConns:          10,
		IdleConnTimeout:       90 * time.Second,
	}
	if opts.Debug != nil {
		transport = newDebugTransport(transport, opts.Debug)
	}
	raw := &http.Client{Transport: transport}

	impl, _ := publicapi.NewClientWithResponses(base,
		publicapi.WithHTTPClient(raw),
		publicapi.WithRequestEditorFn(func(_ context.Context, req *http.Request) error {
			req.Header.Set("Accept", "application/json")
			if apiKey != "" {
				req.Header.Set("Authorization", "Bearer "+apiKey)
			}
			return nil
		}),
	)
	return &Client{impl: impl, base: base, raw: raw, apiKey: apiKey}
}

// idempotencyKey returns a fresh UUID v4 wrapped for direct use as the
// IdempotencyKey field of a generated params struct.
func idempotencyKey() *string {
	s := uuid.NewString()
	return &s
}

// withHeaders is a publicapi.RequestEditorFn that injects extra HTTP
// headers onto a generated request. Used for endpoints that take
// header-keyed inputs the spec doesn't model (X-Move-Source, If-None-
// Match, Last-Modified, Prefer).
func withHeaders(extra map[string]string) publicapi.RequestEditorFn {
	return func(_ context.Context, req *http.Request) error {
		for k, v := range extra {
			req.Header.Set(k, v)
		}
		return nil
	}
}

// hasJSONBody reports whether resp.Body looks like JSON we can decode
// into an ErrorResponse — used by checkStatus when the generated
// JSON{4xx,5xx} fields aren't populated (e.g. unexpected status codes).
func hasJSONBody(b []byte) bool {
	b = []byte(strings.TrimSpace(string(b)))
	return len(b) > 0 && (b[0] == '{' || b[0] == '[')
}

// checkStatus turns a non-2xx response into an *APIError populated from
// the generic ErrorResponse shape (when present). 2xx returns nil.
func checkStatus(statusCode int, body []byte) error {
	if statusCode >= 200 && statusCode < 300 {
		return nil
	}
	apiErr := &APIError{StatusCode: statusCode}
	if hasJSONBody(body) {
		var errBody publicapi.ErrorResponse
		if json.Unmarshal(body, &errBody) == nil {
			apiErr.Code = errBody.Error.Code
			apiErr.Message = errBody.Error.Message
			if errBody.Error.Field != nil {
				apiErr.Field = *errBody.Error.Field
			}
		}
	}
	return apiErr
}

// decodeChecked applies checkStatus to a buffered response and, on success,
// unmarshals the body into a fresh T. Shared by the run/get operations whose
// public-spec responses are typed as freeform objects and re-decoded here.
func decodeChecked[T any](statusCode int, body []byte) (*T, error) {
	if err := checkStatus(statusCode, body); err != nil {
		return nil, err
	}
	var v T
	if err := json.Unmarshal(body, &v); err != nil {
		return nil, err
	}
	return &v, nil
}

// decodeErrorFromHTTPResponse reads and parses an error from a streaming
// *http.Response. The response body is consumed and closed. Returns nil
// if the status code is 2xx.
func decodeErrorFromHTTPResponse(resp *http.Response) error {
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}
	defer func() { _ = resp.Body.Close() }()
	body, _ := io.ReadAll(resp.Body)
	return checkStatus(resp.StatusCode, body)
}

// rawGET issues a GET against an absolute path under the configured
// API base (with /v1 already applied) and returns the raw response.
// Used for /status and for streaming downloads where the generated
// client would needlessly buffer the body.
//
// path may include a leading /v1 (matches the existing call sites) or
// be unversioned (/status). The base already includes /v1, so a leading
// /v1 in path is stripped to avoid a doubled prefix.
func (c *Client) rawGET(ctx context.Context, path string) (*http.Response, error) {
	url, err := c.url(path)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}
	return c.raw.Do(req)
}

// url resolves a CLI-supplied path to an absolute URL. It accepts both
// the legacy /v1/... form (interpreted relative to the unversioned root)
// and unversioned absolute paths like /status.
func (c *Client) url(path string) (string, error) {
	if !strings.HasPrefix(path, "/") {
		return "", fmt.Errorf("path must start with /: %q", path)
	}
	root := strings.TrimSuffix(c.base, "/v1")
	return root + path, nil
}

// errEmptyBody indicates a 2xx response with no decodable body when
// the caller expected one. Most operations treat this as a bug in the
// server contract rather than a transient failure.
var errEmptyBody = errors.New("api: empty response body")
