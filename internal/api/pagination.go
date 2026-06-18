package api

import (
	"net/http"
	"net/url"
	"strings"
)

// paginateAll calls fetchPage repeatedly, threading the rel="next" cursor
// from the previous response's Link header. Stops when the server reports
// no further page (empty cursor) or a duplicate cursor is observed (cycle
// guard against a malformed cursor chain).
//
// fetchPage receives a *string cursor (nil on the first call, then the
// cursor returned by the previous page) and returns the page items plus
// the HTTP response headers so the Link header can be inspected.
func paginateAll[T any](fetchPage func(cursor *string) (items []T, headers http.Header, err error)) ([]T, error) {
	var all []T
	seen := map[string]struct{}{}
	var cursor *string
	for {
		items, headers, err := fetchPage(cursor)
		if err != nil {
			return nil, err
		}
		all = append(all, items...)
		next := nextCursorFromHeaders(headers)
		if next == "" {
			return all, nil
		}
		if _, ok := seen[next]; ok {
			return all, nil
		}
		seen[next] = struct{}{}
		nextCopy := next
		cursor = &nextCopy
	}
}

// maxListPageSize is the server's maximum `limit` for the cursor-paginated
// list endpoints; paginateLimited fetches larger caller limits in pages of
// this size (the server rejects bigger values with a 400 rather than
// clamping).
const maxListPageSize = 200

// paginateLimited fetches up to `limit` items across rel="next" pages, the
// shared loop behind list operations that take a caller limit (SQL
// statements, wp-cli commands, …). limit ≤ 0 fetches a single page at the
// server's default size rather than crawling the whole history; a positive
// limit is fetched in clamped pages and truncated to exactly limit.
//
// fetchPage receives the next cursor (nil on the first call) and the page
// size to request (nil when unbounded) and returns the page items plus the
// response headers so the Link cursor can be threaded. It is responsible for
// its own status check.
func paginateLimited[T any](limit int, fetchPage func(cursor *string, pageSize *int) (items []T, headers http.Header, err error)) ([]T, error) {
	if limit <= 0 {
		items, _, err := fetchPage(nil, nil)
		return items, err
	}

	var all []T
	seen := map[string]struct{}{}
	var cursor *string
	for {
		remaining := limit - len(all)
		if remaining <= 0 {
			return all, nil
		}
		pageSize := remaining
		if pageSize > maxListPageSize {
			pageSize = maxListPageSize
		}
		items, headers, err := fetchPage(cursor, &pageSize)
		if err != nil {
			return nil, err
		}
		all = append(all, items...)
		if len(all) >= limit {
			return all[:limit], nil
		}
		next := nextCursorFromHeaders(headers)
		if next == "" {
			return all, nil
		}
		if _, ok := seen[next]; ok {
			return all, nil
		}
		seen[next] = struct{}{}
		nextCopy := next
		cursor = &nextCopy
	}
}

// nextCursorFromHeaders extracts the cursor query parameter from the
// rel="next" entry of a Link header per RFC 5988. Returns "" when the
// header is absent or no rel="next" entry is present.
func nextCursorFromHeaders(headers http.Header) string {
	if headers == nil {
		return ""
	}
	for _, value := range headers.Values("Link") {
		for _, part := range strings.Split(value, ",") {
			part = strings.TrimSpace(part)
			start := strings.Index(part, "<")
			end := strings.Index(part, ">")
			if start < 0 || end <= start {
				continue
			}
			if !strings.Contains(part[end+1:], `rel="next"`) && !strings.Contains(part[end+1:], `rel=next`) {
				continue
			}
			u, err := url.Parse(part[start+1 : end])
			if err != nil {
				continue
			}
			return u.Query().Get("cursor")
		}
	}
	return ""
}
