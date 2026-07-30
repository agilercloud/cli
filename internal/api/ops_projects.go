package api

import (
	"context"
	"net/http"

	"github.com/agilercloud/cli/internal/publicapi"
)

// ListProjects fetches the caller's projects, optionally scoped to a
// workspace, following Link rel="next" pagination so callers see all
// projects.
func (c *Client) ListProjects(ctx context.Context, workspaceID string) ([]Project, error) {
	workspaceUUID, err := parseOptionalUUID("workspace", workspaceID)
	if err != nil {
		return nil, err
	}
	return paginateAll(func(cursor *string) ([]Project, http.Header, error) {
		params := &publicapi.ListProjectsParams{Cursor: cursor}
		if workspaceUUID != nil {
			params.WorkspaceId = workspaceUUID
		}
		resp, err := c.impl.ListProjectsWithResponse(ctx, params)
		if err != nil {
			return nil, nil, err
		}
		return requireJSONPage(resp.StatusCode(), resp.Body, resp.JSON200, resp.HTTPResponse.Header)
	})
}

// GetProject fetches one project by ID.
func (c *Client) GetProject(ctx context.Context, projectID string) (*ProjectDetail, error) {
	resp, err := c.impl.GetProjectWithResponse(ctx, projectID)
	if err != nil {
		return nil, err
	}
	return requireJSONResponse(resp.StatusCode(), resp.Body, resp.JSON200)
}

// CreateProject creates a project with an auto-generated Idempotency-Key.
func (c *Client) CreateProject(ctx context.Context, in CreateProjectInput) (*ProjectDetail, error) {
	params := &publicapi.CreateProjectParams{IdempotencyKey: idempotencyKey()}
	resp, err := c.impl.CreateProjectWithResponse(ctx, params, in)
	if err != nil {
		return nil, err
	}
	return requireJSONResponse(resp.StatusCode(), resp.Body, resp.JSON201)
}

// UpdateProject patches a project and returns the refreshed entity, so
// callers can render the new state without a follow-up GetProject.
func (c *Client) UpdateProject(ctx context.Context, projectID string, in UpdateProjectInput) (*ProjectDetail, error) {
	resp, err := c.impl.UpdateProjectWithResponse(ctx, projectID, &publicapi.UpdateProjectParams{}, in)
	if err != nil {
		return nil, err
	}
	return requireJSONResponse(resp.StatusCode(), resp.Body, resp.JSON200)
}

// DeleteProject removes a project.
func (c *Client) DeleteProject(ctx context.Context, projectID string) error {
	resp, err := c.impl.DeleteProjectWithResponse(ctx, projectID, &publicapi.DeleteProjectParams{})
	if err != nil {
		return err
	}
	return checkStatus(resp.StatusCode(), resp.Body)
}
