package api

import (
	"context"
	"time"

	"github.com/agilercloud/cli/internal/publicapi"
)

// LogsQuery filters /v1/projects/{id}/logs. Zero-valued fields mean
// "no filter" — friendlier than building a params struct of pointers.
type LogsQuery struct {
	Since time.Time
	Until time.Time
	Query string
	Limit int
}

// GetProjectLogs returns recent logs for a project, optionally filtered.
func (c *Client) GetProjectLogs(ctx context.Context, projectID string, q LogsQuery) ([]LogEntry, error) {
	params := &publicapi.GetProjectLogsParams{}
	if !q.Since.IsZero() {
		s := q.Since
		params.Since = &s
	}
	if !q.Until.IsZero() {
		u := q.Until
		params.Until = &u
	}
	if q.Query != "" {
		qv := q.Query
		params.Q = &qv
	}
	if q.Limit > 0 {
		l := q.Limit
		params.Limit = &l
	}
	resp, err := c.impl.GetProjectLogsWithResponse(ctx, projectID, params)
	if err != nil {
		return nil, err
	}
	if err := checkStatus(resp.StatusCode(), resp.Body); err != nil {
		return nil, err
	}
	if resp.JSON200 == nil {
		return nil, nil
	}
	return *resp.JSON200, nil
}

// UsageQuery filters /v1/projects/{id}/usage.
type UsageQuery struct {
	Since       time.Time
	Until       time.Time
	Granularity UsageGranularity
	Limit       int
}

// GetProjectUsage returns time-bucketed usage records for a project.
func (c *Client) GetProjectUsage(ctx context.Context, projectID string, q UsageQuery) ([]Usage, error) {
	params := &publicapi.GetProjectUsageParams{}
	if !q.Since.IsZero() {
		s := q.Since
		params.Since = &s
	}
	if !q.Until.IsZero() {
		u := q.Until
		params.Until = &u
	}
	if q.Granularity != "" {
		g := q.Granularity
		params.Granularity = &g
	}
	if q.Limit > 0 {
		l := q.Limit
		params.Limit = &l
	}
	resp, err := c.impl.GetProjectUsageWithResponse(ctx, projectID, params)
	if err != nil {
		return nil, err
	}
	if err := checkStatus(resp.StatusCode(), resp.Body); err != nil {
		return nil, err
	}
	if resp.JSON200 == nil {
		return nil, nil
	}
	return *resp.JSON200, nil
}
