package api

import (
	"context"
	"net/http"

	"github.com/agilercloud/cli/internal/publicapi"
)

// ListRuleOptions returns the catalog of fact/operator/value triples
// and rule templates the platform supports. The CLI renders this as
// JSON; there's no narrower view worth materializing.
func (c *Client) ListRuleOptions(ctx context.Context) (*RuleOptions, error) {
	resp, err := c.impl.ListRuleOptionsWithResponse(ctx)
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

// ListProjectRules returns all rules attached to a project, following
// Link rel="next" pagination so callers see the full list.
func (c *Client) ListProjectRules(ctx context.Context, projectID string) ([]Rule, error) {
	return paginateAll(func(cursor *string) ([]Rule, http.Header, error) {
		resp, err := c.impl.ListProjectRulesWithResponse(ctx, projectID, &publicapi.ListProjectRulesParams{Cursor: cursor})
		if err != nil {
			return nil, nil, err
		}
		if err := checkStatus(resp.StatusCode(), resp.Body); err != nil {
			return nil, nil, err
		}
		var items []Rule
		if resp.JSON200 != nil {
			items = *resp.JSON200
		}
		return items, resp.HTTPResponse.Header, nil
	})
}

// CreateRule attaches a new rule to a project.
func (c *Client) CreateRule(ctx context.Context, projectID string, in CreateRuleInput) (*Rule, error) {
	params := &publicapi.CreateProjectRuleParams{IdempotencyKey: idempotencyKey()}
	resp, err := c.impl.CreateProjectRuleWithResponse(ctx, projectID, params, in)
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

// UpdateRule patches an existing rule and returns the refreshed entity.
func (c *Client) UpdateRule(ctx context.Context, projectID, ruleID string, in UpdateRuleInput) (*Rule, error) {
	resp, err := c.impl.UpdateProjectRuleWithResponse(
		ctx, projectID, ruleID, &publicapi.UpdateProjectRuleParams{}, in,
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

// DeleteRule removes a rule.
func (c *Client) DeleteRule(ctx context.Context, projectID, ruleID string) error {
	resp, err := c.impl.DeleteProjectRuleWithResponse(
		ctx, projectID, ruleID, &publicapi.DeleteProjectRuleParams{},
	)
	if err != nil {
		return err
	}
	return checkStatus(resp.StatusCode(), resp.Body)
}
