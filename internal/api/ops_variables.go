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
		if err := checkStatus(resp.StatusCode(), resp.Body); err != nil {
			return nil, nil, err
		}
		var items []Variable
		if resp.JSON200 != nil {
			items = *resp.JSON200
		}
		return items, resp.HTTPResponse.Header, nil
	})
}

// CreateVariable sets a new variable on a project.
func (c *Client) CreateVariable(ctx context.Context, projectID string, in CreateVariableInput) (*Variable, error) {
	params := &publicapi.CreateProjectVariableParams{IdempotencyKey: idempotencyKey()}
	resp, err := c.impl.CreateProjectVariableWithResponse(ctx, projectID, params, in)
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

// UpdateVariable patches an existing variable (e.g. rotate value) and
// returns the refreshed entity.
func (c *Client) UpdateVariable(ctx context.Context, projectID, variableID string, in UpdateVariableInput) (*Variable, error) {
	resp, err := c.impl.UpdateProjectVariableWithResponse(
		ctx, projectID, variableID, &publicapi.UpdateProjectVariableParams{}, in,
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
