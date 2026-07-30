package api

import (
	"context"
	"net/http"

	"github.com/agilercloud/cli/internal/publicapi"
)

// ListVariables returns a project's environment variables, following Link
// rel="next" pagination. Sensitive values come back redacted; the platform
// never serves them on list.
func (c *Client) ListVariables(ctx context.Context, projectID string) ([]Variable, error) {
	return paginateAll(func(cursor *string) ([]Variable, http.Header, error) {
		resp, err := c.impl.ListProjectVariablesWithResponse(ctx, projectID, &publicapi.ListProjectVariablesParams{Cursor: cursor})
		if err != nil {
			return nil, nil, err
		}
		return requireJSONPage(resp.StatusCode(), resp.Body, resp.JSON200, resp.HTTPResponse.Header)
	})
}

// CreateVariable sets a new variable on a project.
func (c *Client) CreateVariable(ctx context.Context, projectID string, in CreateVariableInput) (*Variable, error) {
	params := &publicapi.CreateProjectVariableParams{IdempotencyKey: idempotencyKey()}
	resp, err := c.impl.CreateProjectVariableWithResponse(ctx, projectID, params, in)
	if err != nil {
		return nil, err
	}
	return requireJSONResponse(resp.StatusCode(), resp.Body, resp.JSON201)
}

// UpdateVariable patches an existing variable (e.g. rotate value) and
// returns the refreshed entity.
func (c *Client) UpdateVariable(ctx context.Context, projectID, variableID string, in UpdateVariableInput) (*Variable, error) {
	resp, err := c.impl.UpdateProjectVariableWithResponse(
		ctx, projectID, variableID, &publicapi.UpdateProjectVariableParams{}, in,
	)
	if err != nil {
		return nil, err
	}
	return requireJSONResponse(resp.StatusCode(), resp.Body, resp.JSON200)
}

// DeleteVariable removes a variable.
func (c *Client) DeleteVariable(ctx context.Context, projectID, variableID string) error {
	resp, err := c.impl.DeleteProjectVariableWithResponse(
		ctx, projectID, variableID, &publicapi.DeleteProjectVariableParams{},
	)
	if err != nil {
		return err
	}
	return checkStatus(resp.StatusCode(), resp.Body)
}
