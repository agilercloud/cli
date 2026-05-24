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
