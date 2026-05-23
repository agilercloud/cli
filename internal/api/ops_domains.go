package api

import (
	"context"

	"github.com/agilercloud/cli/internal/publicapi"
)

// ListDomains returns domains attached to a project.
func (c *Client) ListDomains(ctx context.Context, projectID string) ([]Domain, error) {
	resp, err := c.impl.ListProjectDomainsWithResponse(ctx, projectID)
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

// CreateDomain attaches a new custom domain.
func (c *Client) CreateDomain(ctx context.Context, projectID string, in CreateDomainInput) (*Domain, error) {
	params := &publicapi.CreateProjectDomainParams{IdempotencyKey: idempotencyKey()}
	resp, err := c.impl.CreateProjectDomainWithResponse(ctx, projectID, params, in)
	if err != nil {
		return nil, err
	}
	if err := checkStatus(resp.StatusCode(), resp.Body); err != nil {
		return nil, err
	}
	if resp.JSON201 == nil {
		return nil, errEmptyBody
	}
	return resp.JSON201, nil
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
	if err := checkStatus(resp.StatusCode(), resp.Body); err != nil {
		return nil, err
	}
	if resp.JSON200 == nil {
		return nil, errEmptyBody
	}
	return resp.JSON200, nil
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
