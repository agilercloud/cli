package api

import (
	"context"
	"time"

	"github.com/agilercloud/cli/internal/publicapi"
)

// LogsQuery filters /v1/projects/{id}/logs. Zero-valued fields mean
// "no filter" — friendlier than building a params struct of pointers.
type LogsQuery struct {
	Since    time.Time
	Until    time.Time
	Query    string
	Cursor   string
	Limit    int // Total result cap for high-level log reads.
	PageSize int // Per-request cap for callers handling cursors directly.
}

// LogsPage is a cursor-paginated page of project log entries.
type LogsPage struct {
	Items      []LogEntry `json:"items"`
	NextCursor string     `json:"next_cursor,omitempty"`
}

// GetProjectLogsPage returns recent logs for a project, optionally filtered.
func (c *Client) GetProjectLogsPage(ctx context.Context, projectID string, q LogsQuery) (*LogsPage, error) {
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
	if q.Cursor != "" {
		cursor := q.Cursor
		params.Cursor = &cursor
	}
	if q.PageSize > 0 {
		l := q.PageSize
		params.Limit = &l
	}
	resp, err := c.impl.GetProjectLogsWithResponse(ctx, projectID, params)
	if err != nil {
		return nil, err
	}
	if err := checkStatus(resp.StatusCode(), resp.Body); err != nil {
		return nil, err
	}
	page := &LogsPage{
		NextCursor: nextCursorFromHeaders(resp.HTTPResponse.Header),
	}
	if resp.JSON200 == nil {
		return page, nil
	}
	page.Items = *resp.JSON200
	return page, nil
}

// GetProjectLogs returns recent logs for a project, optionally filtered.
func (c *Client) GetProjectLogs(ctx context.Context, projectID string, q LogsQuery) ([]LogEntry, error) {
	var all []LogEntry
	seenCursors := map[string]struct{}{}
	if q.Cursor != "" {
		seenCursors[q.Cursor] = struct{}{}
	}
	remaining := q.Limit
	for {
		pageQuery := q
		if q.Limit > 0 {
			if remaining <= 0 {
				return all, nil
			}
			pageSize := maxLogPageSize
			if q.PageSize > 0 {
				pageSize = min(q.PageSize, maxLogPageSize)
			}
			pageQuery.PageSize = min(remaining, pageSize)
		}
		page, err := c.GetProjectLogsPage(ctx, projectID, pageQuery)
		if err != nil {
			return nil, err
		}
		if page == nil {
			return all, nil
		}
		items := page.Items
		if q.Limit > 0 && len(items) > remaining {
			items = items[:remaining]
		}
		all = append(all, items...)
		if q.Limit > 0 {
			remaining -= len(items)
			if remaining <= 0 {
				return all, nil
			}
		}
		if page.NextCursor == "" {
			return all, nil
		}
		if _, seen := seenCursors[page.NextCursor]; seen {
			return all, nil
		}
		seenCursors[page.NextCursor] = struct{}{}
		q.Cursor = page.NextCursor
	}
}

const maxLogPageSize = 1000

// UsageQuery filters /v1/projects/{id}/usage.
type UsageQuery struct {
	Since       time.Time
	Until       time.Time
	Granularity UsageGranularity
	Limit       int
}

// GetProjectUsage returns time-bucketed usage records for a project.
// The /usage endpoint paginates the bucket list via the Link header
// (totals are window-wide, so the first page is authoritative for the
// totals — they're discarded here, but rebuilt by GetProjectUsageWithTotals).
// This loop follows the cursor chain until exhausted.
func (c *Client) GetProjectUsage(ctx context.Context, projectID string, q UsageQuery) ([]Usage, error) {
	page, err := c.GetProjectUsageWithTotals(ctx, projectID, q)
	if err != nil {
		return nil, err
	}
	return page.Items, nil
}

// UsagePage is a fully-paginated usage response: every bucket across the
// requested window plus the window-wide totals (computed server-side,
// captured from the first page).
type UsagePage struct {
	Items  []Usage
	Totals publicapi.ProjectUsageTotals
}

// GetProjectUsageWithTotals returns both the time-bucketed usage records
// and the window-wide totals, following Link rel="next" pagination.
func (c *Client) GetProjectUsageWithTotals(ctx context.Context, projectID string, q UsageQuery) (*UsagePage, error) {
	out := &UsagePage{}
	seen := map[string]struct{}{}
	var cursor *string
	totalsCaptured := false
	for {
		params := &publicapi.GetProjectUsageParams{Cursor: cursor}
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
		if resp.JSON200 != nil {
			out.Items = append(out.Items, resp.JSON200.Items...)
			if !totalsCaptured {
				out.Totals = resp.JSON200.Totals
				totalsCaptured = true
			}
		}
		next := nextCursorFromHeaders(resp.HTTPResponse.Header)
		if next == "" {
			return out, nil
		}
		if _, ok := seen[next]; ok {
			return out, nil
		}
		seen[next] = struct{}{}
		nextCopy := next
		cursor = &nextCopy
	}
}
