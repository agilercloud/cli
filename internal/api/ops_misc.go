package api

import (
	"context"
	"encoding/json"
	"io"
)

// ListRegions returns the catalog of runtime regions.
func (c *Client) ListRegions(ctx context.Context) ([]Region, error) {
	resp, err := c.impl.ListRegionsWithResponse(ctx)
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

// GetRegion returns a single region by ID.
func (c *Client) GetRegion(ctx context.Context, regionID string) (*Region, error) {
	resp, err := c.impl.GetRegionWithResponse(ctx, regionID)
	if err != nil {
		return nil, err
	}
	if err := checkStatus(resp.StatusCode(), resp.Body); err != nil {
		return nil, err
	}
	if resp.JSON200 == nil {
		return nil, errEmptyBody
	}
	return resp.JSON200, nil
}

// ListRuntimes returns the catalog of runtime images.
func (c *Client) ListRuntimes(ctx context.Context) ([]Runtime, error) {
	resp, err := c.impl.ListRuntimesWithResponse(ctx)
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

// GetRuntime returns a single runtime by ID.
func (c *Client) GetRuntime(ctx context.Context, runtimeID string) (*Runtime, error) {
	resp, err := c.impl.GetRuntimeWithResponse(ctx, runtimeID)
	if err != nil {
		return nil, err
	}
	if err := checkStatus(resp.StatusCode(), resp.Body); err != nil {
		return nil, err
	}
	if resp.JSON200 == nil {
		return nil, errEmptyBody
	}
	return resp.JSON200, nil
}

// GetStatus probes the unversioned /status health endpoint. Doesn't go
// through the generated client because /status sits outside the v1 spec.
func (c *Client) GetStatus(ctx context.Context) (*Status, error) {
	resp, err := c.rawGET(ctx, "/status")
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	body, _ := io.ReadAll(resp.Body)
	if err := checkStatus(resp.StatusCode, body); err != nil {
		return nil, err
	}
	var s Status
	if err := json.Unmarshal(body, &s); err != nil {
		return nil, err
	}
	return &s, nil
}
