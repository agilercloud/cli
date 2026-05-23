package api

import (
	"context"

	"github.com/agilercloud/cli/internal/publicapi"
)

// ListProjects fetches the caller's projects, optionally scoped to a workspace.
func (c *Client) ListProjects(ctx context.Context, workspaceID string) ([]Project, error) {
	var params *publicapi.ListProjectsParams
	if id, err := parseOptionalUUID("workspace", workspaceID); err != nil {
		return nil, err
	} else if id != nil {
		params = &publicapi.ListProjectsParams{WorkspaceId: id}
	}

	resp, err := c.impl.ListProjectsWithResponse(ctx, params)
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

// GetProject fetches one project by ID.
func (c *Client) GetProject(ctx context.Context, projectID string) (*ProjectDetail, error) {
	resp, err := c.impl.GetProjectWithResponse(ctx, projectID)
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

// CreateProject creates a project with an auto-generated Idempotency-Key.
func (c *Client) CreateProject(ctx context.Context, in CreateProjectInput) (*ProjectDetail, error) {
	params := &publicapi.CreateProjectParams{IdempotencyKey: idempotencyKey()}
	resp, err := c.impl.CreateProjectWithResponse(ctx, params, in)
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

// UpdateProject patches a project and returns the refreshed entity, so
// callers can render the new state without a follow-up GetProject.
func (c *Client) UpdateProject(ctx context.Context, projectID string, in UpdateProjectInput) (*ProjectDetail, error) {
	resp, err := c.impl.UpdateProjectWithResponse(ctx, projectID, &publicapi.UpdateProjectParams{}, in)
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

// DeleteProject removes a project.
func (c *Client) DeleteProject(ctx context.Context, projectID string) error {
	resp, err := c.impl.DeleteProjectWithResponse(ctx, projectID, &publicapi.DeleteProjectParams{})
	if err != nil {
		return err
	}
	return checkStatus(resp.StatusCode(), resp.Body)
}
