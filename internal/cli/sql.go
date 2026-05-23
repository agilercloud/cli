package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
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
		Long:  "Execute a SQL statement against the configured project's database. The statement can be passed as an argument or piped via stdin. Use --read-only to wrap execution in a read-only transaction. Use --async to submit without blocking; the command then polls until the statement leaves the pending state or --poll-timeout elapses.",
		Example: `  agiler sql execute "select count(*) from users"
  agiler sql execute --read-only --timeout 30 "select * from orders limit 10"
  echo "vacuum analyze" | agiler sql execute --async`,
		Args: cobra.RangeArgs(0, 1),
		RunE: func(cmd *cobra.Command, args []string) error {
			projectID, err := requireProjectID(a)
			if err != nil {
				return err
			}
			query, err := readSQLQuery(a, args)
			if err != nil {
				return err
			}
			in := api.CreateSQLStatement{Sql: query, ReadOnly: readOnly}
			if timeout > 0 {
				in.Timeout = &timeout
			}

			res, err := a.API.RunSQL(cmd.Context(), projectID, in, async)
			if err != nil {
				return err
			}

			result := res.Statement
			if res.Pending {
				final, err := pollUntilDone(cmd.Context(), a, projectID, result.ID, pollInterval, pollTimeout)
				if err != nil {
					return err
				}
				result = final
			}
			return renderSQLStatement(a, result, async && res.Pending)
		},
	}
	cmd.Flags().BoolVar(&readOnly, "read-only", false, "Wrap execution in a read-only transaction")
	cmd.Flags().IntVar(&timeout, "timeout", 0, "Per-statement timeout in seconds (server-side)")
	cmd.Flags().BoolVar(&async, "async", false, "Send Prefer: respond-async; poll until complete")
	cmd.Flags().DurationVar(&pollInterval, "poll-interval", time.Second, "Poll interval when waiting for async SQL completion")
	cmd.Flags().DurationVar(&pollTimeout, "poll-timeout", 10*time.Minute, "Maximum wait when --async polls for completion")
	return cmd
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
	cmd.Flags().IntVar(&limit, "limit", 0, "Maximum entries returned (0 = server default, max 200)")
	return cmd
}

func newSQLGetCmd(a *app.App) *cobra.Command {
	return &cobra.Command{
		Use:     "get <statement>",
		Short:   "Fetch one SQL execution by id (with paginated rows)",
		Long:    "Fetch a single SQL execution by id, including its full SQL text, row counts, error (if any), and a paginated slice of returned rows.",
		Example: `  agiler sql get 01HXY...`,
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			projectID, err := requireProjectID(a)
			if err != nil {
				return err
			}
			result, err := a.API.GetSQLStatement(cmd.Context(), projectID, args[0])
			if err != nil {
				return err
			}
			return renderSQLStatement(a, *result, false)
		},
	}
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

func readSQLQuery(a *app.App, args []string) (string, error) {
	if len(args) == 1 {
		return args[0], nil
	}
	data, err := io.ReadAll(a.In)
	if err != nil {
		return "", fmt.Errorf("read stdin: %w", err)
	}
	query := strings.TrimSpace(string(data))
	if query == "" {
		return "", fmt.Errorf("no query provided")
	}
	return query, nil
}

// pollUntilDone polls GET /sql/statements/{id} on the given interval until
// status leaves "pending" or timeout elapses. Returns the final statement;
// the caller decides how to render it.
func pollUntilDone(ctx context.Context, a *app.App, projectID, stmtID string, interval, timeout time.Duration) (api.SQLStatement, error) {
	deadline := time.Now().Add(timeout)

	for {
		s, err := a.API.GetSQLStatement(ctx, projectID, stmtID)
		if err != nil {
			return api.SQLStatement{}, err
		}
		if s.Status != "pending" {
			return *s, nil
		}
		if time.Now().After(deadline) {
			return *s, fmt.Errorf("timed out waiting for SQL statement to complete")
		}
		select {
		case <-ctx.Done():
			return *s, ctx.Err()
		case <-time.After(interval):
		}
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

func renderSQLHistory(a *app.App, items []api.SQLStatement) {
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
		rows[i] = []string{
			s.ID,
			s.Status,
			truncateForTable(s.SQL, 60),
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
