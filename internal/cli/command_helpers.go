package cli

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/agilercloud/cli/internal/app"
)

// readArgOrStdin returns the single positional argument when present, else
// the trimmed contents of stdin. `what` names the value for the empty-input
// error ("query", "command"). Shared by `sql execute` and `wp execute`.
func readArgOrStdin(a *app.App, args []string, what string) (string, error) {
	if len(args) == 1 {
		return args[0], nil
	}
	data, err := io.ReadAll(a.In)
	if err != nil {
		return "", fmt.Errorf("read stdin: %w", err)
	}
	s := strings.TrimSpace(string(data))
	if s == "" {
		return "", fmt.Errorf("no %s provided", what)
	}
	return s, nil
}

// pollUntilComplete re-fetches a command on the given interval until its
// status leaves "pending" or the timeout elapses, the shared wait behind the
// synchronous `sql execute` / `wp execute` paths. `label` names the resource
// in the timeout error. fetch returns the latest entity; status reads its
// status field.
func pollUntilComplete[T any](ctx context.Context, interval, timeout time.Duration, label string, fetch func() (*T, error), status func(*T) string) (*T, error) {
	deadline := time.Now().Add(timeout)

	for {
		v, err := fetch()
		if err != nil {
			return nil, err
		}
		if status(v) != "pending" {
			return v, nil
		}
		if time.Now().After(deadline) {
			return v, fmt.Errorf("timed out waiting for %s to complete", label)
		}
		select {
		case <-ctx.Done():
			return v, ctx.Err()
		case <-time.After(interval):
		}
	}
}

// commandExecution describes the mechanical submit/wait/render workflow used
// by SQL and wp-cli. Resource-specific inputs, result types, rendering, and
// terminal validation remain in their respective command files.
type commandExecution[T any] struct {
	Async        bool
	PollInterval time.Duration
	PollTimeout  time.Duration
	Label        string
	Submit       func(context.Context) (*T, bool, error)
	Fetch        func(context.Context, *T) (*T, error)
	Status       func(*T) string
	Render       func(T, bool) error
	Validate     func(T) error
}

func runCommandExecution[T any](ctx context.Context, opts commandExecution[T]) error {
	result, pending, err := opts.Submit(ctx)
	if err != nil {
		return err
	}
	if opts.Async {
		return opts.Render(*result, pending)
	}
	if pending {
		final, err := pollUntilComplete(ctx, opts.PollInterval, opts.PollTimeout, opts.Label,
			func() (*T, error) { return opts.Fetch(ctx, result) }, opts.Status)
		if err != nil {
			return err
		}
		result = final
	}
	if err := opts.Render(*result, false); err != nil {
		return err
	}
	if opts.Validate != nil {
		return opts.Validate(*result)
	}
	return nil
}
