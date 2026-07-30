package api

import (
	"context"

	"github.com/agilercloud/cli/internal/publicapi"
)

// ListWorkspaces fetches workspaces visible to the caller.
func (c *Client) ListWorkspaces(ctx context.Context) ([]Workspace, error) {
	resp, err := c.impl.ListWorkspacesWithResponse(ctx)
	if err != nil {
		return nil, err
	}
	workspaces, err := requireJSONResponse(resp.StatusCode(), resp.Body, resp.JSON200)
	if err != nil {
		return nil, err
	}
	return *workspaces, nil
}

// GetWorkspace fetches one workspace by ID.
func (c *Client) GetWorkspace(ctx context.Context, workspaceID string) (*Workspace, error) {
	resp, err := c.impl.GetWorkspaceWithResponse(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	return requireJSONResponse(resp.StatusCode(), resp.Body, resp.JSON200)
}

// CreateWorkspace creates a workspace with an auto-generated Idempotency-Key.
func (c *Client) CreateWorkspace(ctx context.Context, in CreateWorkspaceInput) (*Workspace, error) {
	params := &publicapi.CreateWorkspaceParams{IdempotencyKey: idempotencyKey()}
	resp, err := c.impl.CreateWorkspaceWithResponse(ctx, params, in)
	if err != nil {
		return nil, err
	}
	return requireJSONResponse(resp.StatusCode(), resp.Body, resp.JSON201)
}

// ListWorkspaceMembers fetches workspace members and pending invites.
func (c *Client) ListWorkspaceMembers(ctx context.Context, workspaceID string) ([]WorkspaceMember, error) {
	resp, err := c.impl.ListWorkspaceMembersWithResponse(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	members, err := requireJSONResponse(resp.StatusCode(), resp.Body, resp.JSON200)
	if err != nil {
		return nil, err
	}
	return *members, nil
}

// GetWorkspaceBillingTransfer fetches the pending billing transfer, if any.
func (c *Client) GetWorkspaceBillingTransfer(ctx context.Context, workspaceID string) (*WorkspaceBillingTransfer, error) {
	resp, err := c.impl.GetWorkspaceBillingTransferWithResponse(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	return requireJSONResponse(resp.StatusCode(), resp.Body, resp.JSON200)
}
