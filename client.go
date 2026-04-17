package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Client is an HTTP client for the Agiler API.
type Client struct {
	BaseURL    string
	APIKey     string
	HTTPClient *http.Client
}

// APIError represents an error response from the API.
type APIError struct {
	StatusCode int
	Message    string
}

func (e *APIError) Error() string {
	// If the API sent a custom message (not the default HTTP status text),
	// use it directly — it's already descriptive.
	if e.Message != "" && e.Message != http.StatusText(e.StatusCode) {
		return e.Message
	}

	// Map generic HTTP status codes to friendly CLI messages.
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

// NewClient creates a new API client.
func NewClient(baseURL, apiKey string) *Client {
	return &Client{
		BaseURL: strings.TrimSuffix(baseURL, "/"),
		APIKey:  apiKey,
		HTTPClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// Do executes an HTTP request against the API.
func (c *Client) Do(method, path string, body io.Reader) (*http.Response, error) {
	url := c.BaseURL + path

	req, err := http.NewRequest(method, url, body)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Accept", "application/json")
	if c.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.APIKey)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode >= 400 {
		defer resp.Body.Close()
		var apiErr APIError
		apiErr.StatusCode = resp.StatusCode
		var errBody struct {
			Message string `json:"message"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&errBody); err == nil {
			apiErr.Message = errBody.Message
		}
		return nil, &apiErr
	}

	return resp, nil
}

// DoRaw executes a request with a custom content type (for file uploads).
func (c *Client) DoRaw(method, path, contentType string, headers map[string]string, body io.Reader) (*http.Response, error) {
	url := c.BaseURL + path

	req, err := http.NewRequest(method, url, body)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Accept", "application/json")
	if c.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.APIKey)
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode >= 400 {
		defer resp.Body.Close()
		var apiErr APIError
		apiErr.StatusCode = resp.StatusCode
		var errBody struct {
			Message string `json:"message"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&errBody); err == nil {
			apiErr.Message = errBody.Message
		}
		return nil, &apiErr
	}

	return resp, nil
}

// DoJSON executes a request and decodes the JSON response into dest.
func (c *Client) DoJSON(method, path string, body io.Reader, dest any) error {
	resp, err := c.Do(method, path, body)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if dest == nil {
		return nil
	}
	return json.NewDecoder(resp.Body).Decode(dest)
}

// DoJSONWithIdempotency executes a request with an idempotency key.
func (c *Client) DoJSONWithIdempotency(method, path string, body io.Reader, dest any) error {
	url := c.BaseURL + path

	req, err := http.NewRequest(method, url, body)
	if err != nil {
		return err
	}

	req.Header.Set("Accept", "application/json")
	if c.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.APIKey)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Idempotency-Key", uuid.New().String())

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		var apiErr APIError
		apiErr.StatusCode = resp.StatusCode
		var errBody struct {
			Message string `json:"message"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&errBody); err == nil {
			apiErr.Message = errBody.Message
		}
		return &apiErr
	}

	if dest == nil {
		return nil
	}
	return json.NewDecoder(resp.Body).Decode(dest)
}
