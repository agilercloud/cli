package api

import (
	"context"
	"net/http"

	"github.com/agilercloud/cli/internal/publicapi"
)

// ListDomains returns domains attached to a project, following Link
// rel="next" pagination so callers see the full list.
func (c *Client) ListDomains(ctx context.Context, projectID string) ([]Domain, error) {
	return paginateAll(func(cursor *string) ([]Domain, http.Header, error) {
		resp, err := c.impl.ListProjectDomainsWithResponse(ctx, projectID, &publicapi.ListProjectDomainsParams{Cursor: cursor})
		if err != nil {
			return nil, nil, err
		}
		if err := checkStatus(resp.StatusCode(), resp.Body); err != nil {
			return nil, nil, err
		}
		var items []Domain
		if resp.JSON200 != nil {
			items = *resp.JSON200
		}
		return items, resp.HTTPResponse.Header, nil
	})
}

// CreateDomain attaches a new custom domain.
func (c *Client) CreateDomain(ctx context.Context, projectID string, in CreateDomainInput) (*Domain, error) {
	params := &publicapi.CreateProjectDomainParams{IdempotencyKey: idempotencyKey()}
	resp, err := c.impl.CreateProjectDomainWithResponse(ctx, projectID, params, in)
	if err != nil {
		return nil, err
	}
	return requireJSONResponse(resp.StatusCode(), resp.Body, resp.JSON201)
}

// UpdateDomain patches a domain (currently: primary flag) and returns the
// refreshed entity.
func (c *Client) UpdateDomain(ctx context.Context, projectID, domainID string, in UpdateDomainInput) (*Domain, error) {
	resp, err := c.impl.UpdateProjectDomainWithResponse(
		ctx, projectID, domainID, &publicapi.UpdateProjectDomainParams{}, in,
	)
	if err != nil {
		return nil, err
	}
	return requireJSONResponse(resp.StatusCode(), resp.Body, resp.JSON200)
}

// DeleteDomain detaches a domain.
func (c *Client) DeleteDomain(ctx context.Context, projectID, domainID string) error {
	resp, err := c.impl.DeleteProjectDomainWithResponse(
		ctx, projectID, domainID, &publicapi.DeleteProjectDomainParams{},
	)
	if err != nil {
		return err
	}
	return checkStatus(resp.StatusCode(), resp.Body)
}
