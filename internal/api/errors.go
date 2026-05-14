package api

import (
	"fmt"
	"net/http"
)

// APIError is the typed error returned by all *Client methods when the
// server responds with a non-2xx status. StatusCode is always populated;
// Code/Message/Field are filled in when the response carries the
// platform's standard error envelope.
type APIError struct {
	StatusCode int
	Code       string
	Message    string
	Field      string
}

func (e *APIError) Error() string {
	if e.Message != "" && e.Message != http.StatusText(e.StatusCode) {
		if e.Field != "" {
			return fmt.Sprintf("%s: %s", e.Field, e.Message)
		}
		return e.Message
	}

	switch e.StatusCode {
	case http.StatusUnauthorized:
		return "not authenticated — verify your API key is correct"
	case http.StatusForbidden:
		return "permission denied"
	case http.StatusNotFound:
		return "not found"
	case http.StatusTooManyRequests:
		return "rate limit exceeded — try again later"
	case http.StatusRequestEntityTooLarge:
		return "request too large"
	case http.StatusInternalServerError:
		return "unexpected server error — try again later"
	case http.StatusServiceUnavailable:
		return "service unavailable — try again later"
	default:
		return fmt.Sprintf("request failed (%d)", e.StatusCode)
	}
}
