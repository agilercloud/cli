package cli

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/agilercloud/cli/internal/api"
	"github.com/agilercloud/cli/internal/app"
	"github.com/spf13/cobra"
)

// newWPCmd builds the `agiler wp ...` subcommand tree.
//
//	agiler wp execute [command] [--timeout=SECONDS] [--async]
//	agiler wp history [--limit=N]
//	agiler wp get <command>
//	agiler wp delete <command>
//
// Each subcommand resolves the project via --project, AGILER_PROJECT_ID, or
// the project_id config key.
func newWPCmd(a *app.App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "wp",
		Short: "Execute wp-cli commands against a project and inspect prior runs",
		Long:  "Execute, list, and inspect wp-cli commands on a project's WordPress installation.",
	}
	cmd.AddCommand(newWPExecuteCmd(a))
	cmd.AddCommand(newWPHistoryCmd(a))
	cmd.AddCommand(newWPGetCmd(a))
	cmd.AddCommand(newWPDeleteCmd(a))
	return cmd
}

func newWPExecuteCmd(a *app.App) *cobra.Command {
	var timeout int
	var async bool
	var pollInterval time.Duration
	var pollTimeout time.Duration

	cmd := &cobra.Command{
		Use:   "execute [command]",
		Short: "Execute a wp-cli command against a project",
		Long:  "Execute a wp-cli command against the configured project's WordPress installation. The command can be passed as an argument or piped via stdin; a leading \"wp\" is optional. Interactive commands (e.g. shell, db cli, --prompt) are not supported. By default the command waits (by polling) until the execution completes or --poll-timeout elapses; use --async to submit and return immediately with the command id.",
		Example: `  agiler wp execute "plugin list --format=json"
  agiler wp execute --timeout 120 "plugin install hello-dolly --activate"
  echo "core update" | agiler wp execute --async`,
		Args: cobra.RangeArgs(0, 1),
		RunE: func(cmd *cobra.Command, args []string) error {
			projectID, err := requireProjectID(a)
			if err != nil {
				return err
			}
			command, err := readWPCommand(a, args)
			if err != nil {
				return err
			}
			in := api.CreateWPCommand{Command: command}
			if timeout > 0 {
				in.Timeout = &timeout
			}

			// Always submit with Prefer: respond-async and poll for the
			// result. A synchronous submit holds the HTTP response open for
			// the whole execution, which the client transport caps at 30s
			// (ResponseHeaderTimeout) — far less than the 180s a wp command
			// may legitimately run. --async only controls whether we wait.
			res, err := a.API.RunWPCommand(cmd.Context(), projectID, in, true)
			if err != nil {
				return err
			}

			result := res.Command
			if async {
				return renderWPCommand(a, result, res.Pending)
			}
			if res.Pending {
				final, err := pollWPUntilDone(cmd.Context(), a, projectID, result.ID, pollInterval, pollTimeout)
				if err != nil {
					return err
				}
				result = final
			}
			return renderWPCommand(a, result, false)
		},
	}
	cmd.Flags().IntVar(&timeout, "timeout", 0, "Per-command timeout in seconds (server-side)")
	cmd.Flags().BoolVar(&async, "async", false, "Submit without waiting; prints the pending command for later `agiler wp get`")
	cmd.Flags().DurationVar(&pollInterval, "poll-interval", time.Second, "Poll interval while waiting for wp-cli completion")
	cmd.Flags().DurationVar(&pollTimeout, "poll-timeout", 10*time.Minute, "Maximum wait for wp-cli completion")
	return cmd
}

func newWPHistoryCmd(a *app.App) *cobra.Command {
	var limit int
	cmd := &cobra.Command{
		Use:   "history",
		Short: "List recent wp-cli executions for a project",
		Long:  "List recent wp-cli executions for the configured project. Each entry shows the command ID, status, the truncated command, when it was submitted, and how long it ran.",
		Example: `  agiler wp history
  agiler wp history --limit 5
  agiler wp history --format json`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			projectID, err := requireProjectID(a)
			if err != nil {
				return err
			}
			result, err := a.API.ListWPCommands(cmd.Context(), projectID, limit)
			if err != nil {
				return err
			}
			renderWPHistory(a, result)
			return nil
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 0, "Maximum entries returned (0 = single page at server default; max 200)")
	return cmd
}

func newWPGetCmd(a *app.App) *cobra.Command {
	var limit int
	var cursor string
	cmd := &cobra.Command{
		Use:   "get <command>",
		Short: "Fetch one wp-cli execution by id (with paginated output)",
		Long:  "Fetch a single wp-cli execution by id, including its full command text, exit code, error (if any), and a page of output lines. When more output remains, the rendered result ends with the `--cursor` invocation that fetches the next page.",
		Example: `  agiler wp get 01HXY...
  agiler wp get 01HXY... --limit 1000`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			projectID, err := requireProjectID(a)
			if err != nil {
				return err
			}
			result, err := a.API.GetWPCommand(cmd.Context(), projectID, args[0], limit, cursor)
			if err != nil {
				return err
			}
			return renderWPCommand(a, *result, false)
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 0, "Maximum output lines returned (0 = server default of 100, max 1000)")
	cmd.Flags().StringVar(&cursor, "cursor", "", "Resume output from a previous page's cursor")
	return cmd
}

func newWPDeleteCmd(a *app.App) *cobra.Command {
	return &cobra.Command{
		Use:   "delete <command>",
		Short: "Delete a command (cancels it if it's still pending)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			projectID, err := requireProjectID(a)
			if err != nil {
				return err
			}
			if err := a.API.DeleteWPCommand(cmd.Context(), projectID, args[0]); err != nil {
				return err
			}
			a.Output.Text("Command deleted.")
			return nil
		},
	}
}

func readWPCommand(a *app.App, args []string) (string, error) {
	if len(args) == 1 {
		return args[0], nil
	}
	data, err := io.ReadAll(a.In)
	if err != nil {
		return "", fmt.Errorf("read stdin: %w", err)
	}
	command := strings.TrimSpace(string(data))
	if command == "" {
		return "", fmt.Errorf("no command provided")
	}
	return command, nil
}

// pollWPUntilDone polls GET /wp/commands/{id} on the given interval until
// status leaves "pending" or timeout elapses. Returns the final command;
// the caller decides how to render it. Each poll requests the maximum
// output page — free while the command is pending (no output exists yet)
// and it makes the final poll return the largest first page in one go.
func pollWPUntilDone(ctx context.Context, a *app.App, projectID, cmdID string, interval, timeout time.Duration) (api.WPCommand, error) {
	deadline := time.Now().Add(timeout)

	for {
		w, err := a.API.GetWPCommand(ctx, projectID, cmdID, api.MaxWPOutputPageSize, "")
		if err != nil {
			return api.WPCommand{}, err
		}
		if w.Status != "pending" {
			return *w, nil
		}
		if time.Now().After(deadline) {
			return *w, fmt.Errorf("timed out waiting for wp-cli command to complete")
		}
		select {
		case <-ctx.Done():
			return *w, ctx.Err()
		case <-time.After(interval):
		}
	}
}

func renderWPCommand(a *app.App, w api.WPCommand, asyncHint bool) error {
	if a.Output.IsTabular() {
		return tabularUnsupportedErr(a.Output)
	}
	if a.Output.IsStructured() {
		a.Output.Structured(w)
		return nil
	}

	if a.Output.IsQuiet() {
		a.Output.Text("%s", w.ID)
		return nil
	}

	a.Output.Text("ID:           %s", w.ID)
	a.Output.Text("Status:       %s", w.Status)
	if w.SubmittedAt != "" {
		a.Output.Text("Submitted:    %s", w.SubmittedAt)
	}
	if w.CompletedAt != "" {
		a.Output.Text("Completed:    %s", w.CompletedAt)
	}
	if w.DurationMs != nil {
		a.Output.Text("Duration:     %dms", *w.DurationMs)
	}
	if w.ExitCode != nil {
		a.Output.Text("Exit code:    %d", *w.ExitCode)
	}
	if w.LineCount != nil {
		a.Output.Text("Line count:   %d", *w.LineCount)
	}
	if w.Error != nil {
		a.Output.Text("Error:        %s", *w.Error)
	}
	if asyncHint && w.Status == "pending" {
		a.Output.Text("Poll: agiler wp get %s", w.ID)
	}
	for _, line := range w.Output {
		a.Output.Text("%s", line)
	}
	if w.NextCursor != "" {
		a.Output.Text("(more output: agiler wp get %s --cursor %s)", w.ID, w.NextCursor)
	}
	return nil
}

func renderWPHistory(a *app.App, items []api.WPCommandListItem) {
	if a.Output.IsStructured() {
		a.Output.Structured(items)
		return
	}
	if len(items) == 0 {
		a.Output.Text("No wp-cli history.")
		return
	}
	rows := make([][]string, len(items))
	for i, w := range items {
		duration := ""
		if w.DurationMs != nil {
			duration = fmt.Sprintf("%dms", *w.DurationMs)
		}
		// CommandPreview is already truncated server-side to 80 runes
		// (ending in "…" if longer); the table column then trims further
		// to fit.
		rows[i] = []string{
			w.Id,
			w.Status,
			truncateForTable(w.CommandPreview, 60),
			w.SubmittedAt,
			duration,
		}
	}
	a.Output.Table(
		[]string{"ID", "STATUS", "COMMAND", "SUBMITTED", "DURATION"},
		rows,
	)
}
