package cli

import (
	"context"
	"errors"
	"time"

	"github.com/agilercloud/cli/internal/api"
	"github.com/agilercloud/cli/internal/app"
	"github.com/agilercloud/cli/internal/output"
)

const (
	logTailPageSize  = 1000
	logTailSeenLimit = 5000
)

// LogsTailOptions contains the parsed values needed by the logs tail
// workflow. Command construction and signal wiring stay at the Cobra edge.
type LogsTailOptions struct {
	ProjectID string
	Since     time.Time
	Interval  time.Duration
}

type logsPageFetcher func(context.Context, api.LogsQuery) (*api.LogsPage, error)

// logTailer owns one polling window: cursor traversal, duplicate suppression,
// output, and advancement of the next window's lower bound.
type logTailer struct {
	fetch logsPageFetcher
	out   *output.Writer
	since time.Time
	seen  map[string]struct{}
}

func newLogTailer(a *app.App, opts LogsTailOptions) *logTailer {
	return &logTailer{
		fetch: func(ctx context.Context, q api.LogsQuery) (*api.LogsPage, error) {
			return a.API.GetProjectLogsPage(ctx, opts.ProjectID, q)
		},
		out:   a.Output,
		since: opts.Since,
		seen:  map[string]struct{}{},
	}
}

// poll fetches and renders every page in the current time window. Duplicate
// cursors terminate the window, protecting tail from a malformed cursor loop.
func (t *logTailer) poll(ctx context.Context) error {
	cursor := ""
	seenCursors := map[string]struct{}{}
	for {
		page, err := t.fetch(ctx, api.LogsQuery{
			Since:    t.since,
			Cursor:   cursor,
			PageSize: logTailPageSize,
		})
		if err != nil {
			return err
		}
		if page == nil {
			break
		}

		for _, entry := range page.Items {
			key := entry.RequestId.String() + entry.Message
			if _, duplicate := t.seen[key]; duplicate {
				continue
			}
			t.seen[key] = struct{}{}
			renderTailedLog(t.out, entry)
			if entry.Timestamp.After(t.since) {
				t.since = entry.Timestamp.Add(time.Millisecond)
			}
		}

		if page.NextCursor == "" {
			break
		}
		if _, duplicate := seenCursors[page.NextCursor]; duplicate {
			break
		}
		seenCursors[page.NextCursor] = struct{}{}
		cursor = page.NextCursor
	}

	if len(t.seen) > logTailSeenLimit {
		t.seen = map[string]struct{}{}
	}
	return nil
}

func renderTailedLog(w *output.Writer, entry api.LogEntry) {
	switch {
	case w.Format == output.FormatYAML:
		w.Text("---")
		w.Structured(entry)
	case w.IsStructured():
		w.Structured(entry)
	default:
		w.Text("[%s] %s: %s", entry.Timestamp.Format(time.RFC3339), entry.Priority, entry.Message)
	}
}

// runLogsTail polls until its context is canceled. Cancellation is a clean
// terminal condition for a streaming command.
func runLogsTail(ctx context.Context, a *app.App, opts LogsTailOptions) error {
	tailer := newLogTailer(a, opts)
	for {
		if err := tailer.poll(ctx); err != nil {
			if ctx.Err() != nil && errors.Is(err, context.Canceled) {
				return nil
			}
			return err
		}
		select {
		case <-ctx.Done():
			return nil
		case <-a.Clock.After(opts.Interval):
		}
	}
}
