package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/agilercloud/cli/internal/api"
	"github.com/agilercloud/cli/internal/clock"
	"github.com/agilercloud/cli/internal/output"
	"github.com/google/uuid"
)

func TestLogTailerPollPaginatesDeduplicatesAndAdvances(t *testing.T) {
	a, out, _ := newTestApp(t)
	start := time.Date(2026, 7, 29, 12, 0, 0, 0, time.UTC)
	first := logEntry("00000000-0000-0000-0000-000000000001", start.Add(time.Second), "first")
	second := logEntry("00000000-0000-0000-0000-000000000002", start.Add(2*time.Second), "second")

	var queries []api.LogsQuery
	tailer := &logTailer{
		out:   a.Output,
		since: start,
		seen:  map[string]struct{}{},
	}
	tailer.fetch = func(_ context.Context, q api.LogsQuery) (*api.LogsPage, error) {
		queries = append(queries, q)
		switch len(queries) {
		case 1:
			return &api.LogsPage{Items: []api.LogEntry{first}, NextCursor: "page-2"}, nil
		case 2:
			// The repeated cursor ends this polling window after this page.
			return &api.LogsPage{Items: []api.LogEntry{first, second}, NextCursor: "page-2"}, nil
		default:
			t.Fatalf("unexpected fetch %d", len(queries))
			return nil, nil
		}
	}

	if err := tailer.poll(context.Background()); err != nil {
		t.Fatalf("poll: %v", err)
	}
	if len(queries) != 2 {
		t.Fatalf("fetches = %d, want 2", len(queries))
	}
	if queries[0].Cursor != "" || queries[1].Cursor != "page-2" {
		t.Fatalf("cursors = %q, %q", queries[0].Cursor, queries[1].Cursor)
	}
	if queries[0].PageSize != logTailPageSize || queries[1].PageSize != logTailPageSize {
		t.Fatalf("page sizes = %d, %d", queries[0].PageSize, queries[1].PageSize)
	}
	if !queries[1].Since.Equal(first.Timestamp.Add(time.Millisecond)) {
		t.Fatalf("second-page since = %v, want %v", queries[1].Since, first.Timestamp.Add(time.Millisecond))
	}
	if !tailer.since.Equal(second.Timestamp.Add(time.Millisecond)) {
		t.Fatalf("next since = %v, want %v", tailer.since, second.Timestamp.Add(time.Millisecond))
	}
	if got := out.String(); strings.Count(got, "first") != 1 || strings.Count(got, "second") != 1 {
		t.Fatalf("deduplicated output = %q", got)
	}
}

func TestLogTailerPollResetsLargeSeenSet(t *testing.T) {
	a, _, _ := newTestApp(t)
	tailer := &logTailer{
		out:   a.Output,
		since: time.Now(),
		seen:  make(map[string]struct{}, logTailSeenLimit+1),
		fetch: func(context.Context, api.LogsQuery) (*api.LogsPage, error) {
			return &api.LogsPage{}, nil
		},
	}
	for i := 0; i <= logTailSeenLimit; i++ {
		tailer.seen[string(rune(i))] = struct{}{}
	}

	if err := tailer.poll(context.Background()); err != nil {
		t.Fatalf("poll: %v", err)
	}
	if len(tailer.seen) != 0 {
		t.Fatalf("seen size = %d, want reset", len(tailer.seen))
	}
}

func TestRenderTailedLogFormats(t *testing.T) {
	entry := logEntry("00000000-0000-0000-0000-000000000001", time.Date(2026, 7, 29, 12, 0, 0, 0, time.UTC), "hello")
	tests := []struct {
		name   string
		format output.Format
		check  func(*testing.T, string)
	}{
		{name: "text", format: output.FormatText, check: func(t *testing.T, got string) {
			if got != "[2026-07-29T12:00:00Z] INFO: hello\n" {
				t.Fatalf("text output = %q", got)
			}
		}},
		{name: "json", format: output.FormatJSON, check: func(t *testing.T, got string) {
			var decoded api.LogEntry
			if err := json.Unmarshal([]byte(got), &decoded); err != nil {
				t.Fatalf("invalid JSON %q: %v", got, err)
			}
			if decoded.Message != "hello" {
				t.Fatalf("decoded message = %q", decoded.Message)
			}
		}},
		{name: "yaml", format: output.FormatYAML, check: func(t *testing.T, got string) {
			if !strings.HasPrefix(got, "---\n") || !strings.Contains(got, "message: hello") {
				t.Fatalf("YAML output = %q", got)
			}
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var out bytes.Buffer
			w := output.New(tt.format, false, &out, &bytes.Buffer{})
			renderTailedLog(w, entry)
			tt.check(t, out.String())
		})
	}
}

func TestRunLogsTailUsesClockAndCancelsCleanly(t *testing.T) {
	requests := make(chan struct{}, 4)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests <- struct{}{}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[]`))
	}))
	t.Cleanup(server.Close)

	a, _, _ := newTestApp(t)
	fakeClock := clock.NewFake(time.Date(2026, 7, 29, 12, 0, 0, 0, time.UTC))
	a.Clock = fakeClock
	a.API = api.NewClient(server.URL, "test-key", api.Options{})
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- runLogsTail(ctx, a, LogsTailOptions{
			ProjectID: "proj-1",
			Since:     fakeClock.Now(),
			Interval:  2 * time.Second,
		})
	}()

	waitForLogRequest(t, requests)
	advanceFakeClockUntilRequest(t, fakeClock, 2*time.Second, requests)
	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("runLogsTail cancellation: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("runLogsTail did not stop after cancellation")
	}
}

func TestRunLogsQueryPreservesStreamingAndStructuredShapes(t *testing.T) {
	for _, format := range []output.Format{output.FormatText, output.FormatJSON} {
		t.Run(string(format), func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				switch r.URL.Query().Get("cursor") {
				case "":
					w.Header().Set("Link", `</v1/projects/proj-1/logs?cursor=page-2>; rel="next"`)
					_, _ = w.Write([]byte(`[{"request_id":"00000000-0000-0000-0000-000000000001","timestamp":"2026-07-29T12:00:00Z","priority":"INFO","message":"first"}]`))
				case "page-2":
					_, _ = w.Write([]byte(`[{"request_id":"00000000-0000-0000-0000-000000000002","timestamp":"2026-07-29T12:00:01Z","priority":"ERROR","message":"second"}]`))
				default:
					http.NotFound(w, r)
				}
			}))
			t.Cleanup(server.Close)

			a, out, errOut := newTestApp(t)
			a.API = api.NewClient(server.URL, "test-key", api.Options{})
			a.Output = output.New(format, false, out, errOut)
			if err := runLogsQuery(context.Background(), a, "proj-1", api.LogsQuery{}); err != nil {
				t.Fatalf("runLogsQuery: %v", err)
			}
			if format == output.FormatText {
				if got := out.String(); !strings.Contains(got, "first") || !strings.Contains(got, "second") {
					t.Fatalf("text output:\n%s", got)
				}
				return
			}
			var entries []api.LogEntry
			if err := json.Unmarshal(out.Bytes(), &entries); err != nil {
				t.Fatalf("structured output is not one JSON value: %v\n%s", err, out.String())
			}
			if len(entries) != 2 || entries[0].Message != "first" || entries[1].Message != "second" {
				t.Fatalf("structured entries = %#v", entries)
			}
		})
	}
}

func TestLogsTailRejectsInvalidTimeFlags(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{name: "interval", args: []string{"--interval=eventually"}, want: "invalid interval"},
		{name: "since", args: []string{"--since=last-tuesday"}, want: "invalid --since"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a, _, _ := newTestApp(t)
			cmd := newLogsTailCmd(a)
			cmd.SetArgs(tt.args)
			err := cmd.ExecuteContext(context.Background())
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error = %v, want containing %q", err, tt.want)
			}
		})
	}
}

func waitForLogRequest(t *testing.T, requests <-chan struct{}) {
	t.Helper()
	select {
	case <-requests:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for log request")
	}
}

func advanceFakeClockUntilRequest(t *testing.T, fakeClock *clock.Fake, interval time.Duration, requests <-chan struct{}) {
	t.Helper()
	deadline := time.NewTimer(time.Second)
	defer deadline.Stop()
	for {
		fakeClock.Advance(interval)
		select {
		case <-requests:
			return
		case <-deadline.C:
			t.Fatal("timed out waiting for log request after advancing fake clock")
		case <-time.After(time.Millisecond):
		}
	}
}

func logEntry(id string, timestamp time.Time, message string) api.LogEntry {
	return api.LogEntry{
		RequestId: uuid.MustParse(id),
		Timestamp: timestamp,
		Priority:  "INFO",
		Message:   message,
	}
}
