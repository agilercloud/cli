package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/agilercloud/cli/internal/api"
	"github.com/agilercloud/cli/internal/app"
	"github.com/spf13/cobra"
)

// newSQLCmd builds the `agiler sql ...` subcommand tree.
//
//	agiler sql execute [query] [--read-only] [--timeout=SECONDS] [--async]
//	agiler sql history [--limit=N]
//	agiler sql get <statement>
//	agiler sql delete <statement>
//
// Each subcommand resolves the project via --project, AGILER_PROJECT_ID, or
// the project_id config key.
func newSQLCmd(a *app.App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "sql",
		Short: "Execute SQL against a project database and inspect prior runs",
		Long:  "Execute, list, and inspect SQL statements on a project database.",
	}
	cmd.AddCommand(newSQLExecuteCmd(a))
	cmd.AddCommand(newSQLHistoryCmd(a))
	cmd.AddCommand(newSQLGetCmd(a))
	cmd.AddCommand(newSQLDeleteCmd(a))
	return cmd
}

func newSQLExecuteCmd(a *app.App) *cobra.Command {
	var readOnly bool
	var timeout int
	var async bool
	var pollInterval time.Duration
	var pollTimeout time.Duration

	cmd := &cobra.Command{
		Use:   "execute [query]",
		Short: "Execute a SQL statement against a project database",
		Long:  "Execute a SQL statement against the configured project's database. The statement can be passed as an argument or piped via stdin. Use --read-only to wrap execution in a read-only transaction. By default the command waits (by polling) until the statement completes or --poll-timeout elapses; use --async to submit and return immediately with the statement id.",
		Example: `  agiler sql execute "select count(*) from users"
  agiler sql execute --read-only --timeout 30 "select * from orders limit 10"
  echo "optimize table wp_posts" | agiler sql execute --async`,
		Args: cobra.RangeArgs(0, 1),
		RunE: func(cmd *cobra.Command, args []string) error {
			projectID, err := requireProjectID(a)
			if err != nil {
				return err
			}
			query, err := readArgOrStdin(a, args, "query")
			if err != nil {
				return err
			}
			return runSQLExecute(cmd.Context(), a, SQLExecuteOptions{
				ProjectID:    projectID,
				Query:        query,
				ReadOnly:     readOnly,
				Timeout:      timeout,
				Async:        async,
				PollInterval: pollInterval,
				PollTimeout:  pollTimeout,
			})
		},
	}
	cmd.Flags().BoolVar(&readOnly, "read-only", false, "Wrap execution in a read-only transaction")
	cmd.Flags().IntVar(&timeout, "timeout", 0, "Per-statement timeout in seconds (server-side)")
	cmd.Flags().BoolVar(&async, "async", false, "Submit without waiting; prints the pending statement for later `agiler sql get`")
	cmd.Flags().DurationVar(&pollInterval, "poll-interval", time.Second, "Poll interval while waiting for SQL completion")
	cmd.Flags().DurationVar(&pollTimeout, "poll-timeout", 10*time.Minute, "Maximum wait for SQL completion")
	return cmd
}

// SQLExecuteOptions contains the parsed inputs for a SQL execution.
type SQLExecuteOptions struct {
	ProjectID    string
	Query        string
	ReadOnly     bool
	Timeout      int
	Async        bool
	PollInterval time.Duration
	PollTimeout  time.Duration
}

func runSQLExecute(ctx context.Context, a *app.App, opts SQLExecuteOptions) error {
	in := api.CreateSQLStatement{Sql: opts.Query, ReadOnly: opts.ReadOnly}
	if opts.Timeout > 0 {
		in.Timeout = &opts.Timeout
	}

	return runCommandExecution(ctx, commandExecution[api.SQLStatement]{
		Async:        opts.Async,
		PollInterval: opts.PollInterval,
		PollTimeout:  opts.PollTimeout,
		Label:        "SQL statement",
		// Always submit with Prefer: respond-async. --async only controls
		// whether the CLI waits for the result.
		Submit: func(ctx context.Context) (*api.SQLStatement, bool, error) {
			result, err := a.API.RunSQL(ctx, opts.ProjectID, in, true)
			if err != nil {
				return nil, false, err
			}
			return &result.Statement, result.Pending, nil
		},
		Fetch: func(ctx context.Context, submitted *api.SQLStatement) (*api.SQLStatement, error) {
			return a.API.GetSQLStatement(ctx, opts.ProjectID, submitted.ID, api.MaxSQLRowsPageSize, "")
		},
		Status: func(statement *api.SQLStatement) string { return statement.Status },
		Render: func(statement api.SQLStatement, asyncHint bool) error {
			return renderSQLStatement(a, statement, asyncHint)
		},
	})
}

func newSQLHistoryCmd(a *app.App) *cobra.Command {
	var limit int
	cmd := &cobra.Command{
		Use:   "history",
		Short: "List recent SQL executions for a project",
		Long:  "List recent SQL executions for the configured project. Each entry shows the statement ID, status, the truncated SQL, when it was submitted, and how long it ran.",
		Example: `  agiler sql history
  agiler sql history --limit 5
  agiler sql history --format json`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			projectID, err := requireProjectID(a)
			if err != nil {
				return err
			}
			result, err := a.API.ListSQLStatements(cmd.Context(), projectID, limit)
			if err != nil {
				return err
			}
			renderSQLHistory(a, result)
			return nil
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 0, "Maximum entries returned (0 = single page at server default; max 200)")
	return cmd
}

func newSQLGetCmd(a *app.App) *cobra.Command {
	var limit int
	var cursor string
	cmd := &cobra.Command{
		Use:   "get <statement>",
		Short: "Fetch one SQL execution by id (with paginated rows)",
		Long:  "Fetch a single SQL execution by id, including its full SQL text, row counts, error (if any), and a page of returned rows. When more rows remain, the rendered result ends with the `--cursor` invocation that fetches the next page.",
		Example: `  agiler sql get 01HXY...
  agiler sql get 01HXY... --limit 1000`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			projectID, err := requireProjectID(a)
			if err != nil {
				return err
			}
			result, err := a.API.GetSQLStatement(cmd.Context(), projectID, args[0], limit, cursor)
			if err != nil {
				return err
			}
			return renderSQLStatement(a, *result, false)
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 0, "Maximum result rows returned (0 = server default of 100, max 1000)")
	cmd.Flags().StringVar(&cursor, "cursor", "", "Resume rows from a previous page's cursor")
	return cmd
}

func newSQLDeleteCmd(a *app.App) *cobra.Command {
	return &cobra.Command{
		Use:   "delete <statement>",
		Short: "Delete a statement (cancels SQL if it's still pending)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			projectID, err := requireProjectID(a)
			if err != nil {
				return err
			}
			if err := a.API.DeleteSQLStatement(cmd.Context(), projectID, args[0]); err != nil {
				return err
			}
			a.Output.Text("Statement deleted.")
			return nil
		},
	}
}

func renderSQLStatement(a *app.App, s api.SQLStatement, asyncHint bool) error {
	if a.Output.IsTabular() {
		return tabularUnsupportedErr(a.Output)
	}
	if a.Output.IsStructured() {
		a.Output.Structured(s)
		return nil
	}

	if a.Output.IsQuiet() {
		a.Output.Text("%s", s.ID)
		return nil
	}

	a.Output.Text("ID:           %s", s.ID)
	a.Output.Text("Status:       %s", s.Status)
	if s.SubmittedAt != "" {
		a.Output.Text("Submitted:    %s", s.SubmittedAt)
	}
	if s.CompletedAt != "" {
		a.Output.Text("Completed:    %s", s.CompletedAt)
	}
	if s.DurationMs != nil {
		a.Output.Text("Duration:     %dms", *s.DurationMs)
	}
	if s.RowCount != nil {
		a.Output.Text("Row count:    %d", *s.RowCount)
	}
	if s.RowsAffected != nil {
		a.Output.Text("Rows affected: %d", *s.RowsAffected)
	}
	if s.Error != nil {
		a.Output.Text("Error:        %s", *s.Error)
	}
	if asyncHint && s.Status == "pending" {
		a.Output.Text("Poll: agiler sql get %s", s.ID)
	}
	if len(s.Rows) > 0 {
		renderRowsTable(a, s.Columns, s.Rows)
	}
	if s.NextCursor != "" {
		a.Output.Text("(more rows: agiler sql get %s --cursor %s)", s.ID, s.NextCursor)
	}
	return nil
}

func renderRowsTable(a *app.App, columns []string, rows [][]any) {
	if len(columns) == 0 {
		return
	}
	tableRows := make([][]string, len(rows))
	for i, row := range rows {
		cells := make([]string, len(columns))
		for j := range columns {
			if j < len(row) {
				cells[j] = formatCell(row[j])
			}
		}
		tableRows[i] = cells
	}
	a.Output.Table(columns, tableRows)
}

// formatCell renders a single JSON value as a string for tabular output.
// Numbers stay numeric, nil becomes empty, everything else uses fmt.
func formatCell(v any) string {
	if v == nil {
		return ""
	}
	switch x := v.(type) {
	case string:
		return x
	case float64:
		// JSON numbers decode as float64; print integers without decimal.
		if x == float64(int64(x)) {
			return fmt.Sprintf("%d", int64(x))
		}
		return fmt.Sprintf("%g", x)
	case bool:
		return fmt.Sprintf("%t", x)
	default:
		b, err := json.Marshal(x)
		if err != nil {
			return fmt.Sprintf("%v", x)
		}
		return string(b)
	}
}

func renderSQLHistory(a *app.App, items []api.SQLStatementListItem) {
	if a.Output.IsStructured() {
		a.Output.Structured(items)
		return
	}
	if len(items) == 0 {
		a.Output.Text("No SQL history.")
		return
	}
	rows := make([][]string, len(items))
	for i, s := range items {
		duration := ""
		if s.DurationMs != nil {
			duration = fmt.Sprintf("%dms", *s.DurationMs)
		}
		// SqlPreview is already truncated server-side to 80 runes (ending
		// in "…" if longer); the table column then trims further to fit.
		rows[i] = []string{
			s.Id,
			s.Status,
			truncateForTable(s.SqlPreview, 60),
			s.SubmittedAt,
			duration,
		}
	}
	a.Output.Table(
		[]string{"ID", "STATUS", "SQL", "SUBMITTED", "DURATION"},
		rows,
	)
}

// truncateForTable trims long SQL strings to keep the history table
// readable. Multi-line statements collapse to a single line first.
func truncateForTable(s string, n int) string {
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.Join(strings.Fields(s), " ")
	if len(s) <= n {
		return s
	}
	return s[:n-1] + "…"
}
