package cli

import (
	"context"
	"fmt"
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
			command, err := readArgOrStdin(a, args, "command")
			if err != nil {
				return err
			}
			return runWPExecute(cmd.Context(), a, WPExecuteOptions{
				ProjectID:    projectID,
				Command:      command,
				Timeout:      timeout,
				Async:        async,
				PollInterval: pollInterval,
				PollTimeout:  pollTimeout,
			})
		},
	}
	cmd.Flags().IntVar(&timeout, "timeout", 0, "Per-command timeout in seconds (server-side)")
	cmd.Flags().BoolVar(&async, "async", false, "Submit without waiting; prints the pending command for later `agiler wp get`")
	cmd.Flags().DurationVar(&pollInterval, "poll-interval", time.Second, "Poll interval while waiting for wp-cli completion")
	cmd.Flags().DurationVar(&pollTimeout, "poll-timeout", 10*time.Minute, "Maximum wait for wp-cli completion")
	return cmd
}

// WPExecuteOptions contains the parsed inputs for a wp-cli execution.
type WPExecuteOptions struct {
	ProjectID    string
	Command      string
	Timeout      int
	Async        bool
	PollInterval time.Duration
	PollTimeout  time.Duration
}

func runWPExecute(ctx context.Context, a *app.App, opts WPExecuteOptions) error {
	in := api.CreateWPCommand{Command: opts.Command}
	if opts.Timeout > 0 {
		in.Timeout = &opts.Timeout
	}

	return runCommandExecution(ctx, commandExecution[api.WPCommand]{
		Async:        opts.Async,
		PollInterval: opts.PollInterval,
		PollTimeout:  opts.PollTimeout,
		Label:        "wp-cli command",
		// Always submit with Prefer: respond-async. --async only controls
		// whether the CLI waits for the result.
		Submit: func(ctx context.Context) (*api.WPCommand, bool, error) {
			result, err := a.API.RunWPCommand(ctx, opts.ProjectID, in, true)
			if err != nil {
				return nil, false, err
			}
			return &result.Command, result.Pending, nil
		},
		Fetch: func(ctx context.Context, submitted *api.WPCommand) (*api.WPCommand, error) {
			return a.API.GetWPCommand(ctx, opts.ProjectID, submitted.ID, api.MaxWPOutputPageSize, "")
		},
		Status: func(command *api.WPCommand) string { return command.Status },
		Render: func(command api.WPCommand, asyncHint bool) error {
			return renderWPCommand(a, command, asyncHint)
		},
		Validate: wpCommandExecutionError,
	})
}

func wpCommandExecutionError(w api.WPCommand) error {
	if w.Status != "error" {
		return nil
	}
	if w.ExitCode != nil {
		return fmt.Errorf("wp-cli command failed with exit code %d", *w.ExitCode)
	}
	return fmt.Errorf("wp-cli command failed")
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
