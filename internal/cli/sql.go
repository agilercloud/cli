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
//	agiler sql execute <project> [query] [--read-only] [--timeout=SECONDS] [--async]
//	agiler sql history <project> [--limit=N]
//	agiler sql get <project> <statement>
//	agiler sql delete <project> <statement>
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

	cmd := &cobra.Command{
		Use:   "execute <project> [query]",
		Short: "Execute a SQL statement against a project database",
		Long:  "Execute SQL. Provide the statement as an argument, or pipe it via stdin.",
		Args:  cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			query, err := readSQLQuery(a, args)
			if err != nil {
				return err
			}
			in := api.CreateSQLStatement{Sql: query, ReadOnly: readOnly}
			if timeout > 0 {
				in.Timeout = &timeout
			}

			projectID := args[0]
			res, err := a.API.RunSQL(cmd.Context(), projectID, in, async)
			if err != nil {
				return err
			}

			result := res.Statement
			if res.Pending {
				final, err := pollUntilDone(cmd.Context(), a, projectID, result.ID)
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
	return cmd
}

func newSQLHistoryCmd(a *app.App) *cobra.Command {
	var limit int
	cmd := &cobra.Command{
		Use:     "history <project>",
		Aliases: []string{"list", "ls"},
		Short:   "List recent SQL executions for a project",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			result, err := a.API.ListSQLStatements(cmd.Context(), args[0], limit)
			if err != nil {
				return err
			}
			renderSQLHistory(a, result)
			return nil
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 0, "Maximum number of history entries (server default 50, max 200)")
	return cmd
}

func newSQLGetCmd(a *app.App) *cobra.Command {
	return &cobra.Command{
		Use:   "get <project> <statement>",
		Short: "Fetch one SQL execution by id (with paginated rows)",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			result, err := a.API.GetSQLStatement(cmd.Context(), args[0], args[1])
			if err != nil {
				return err
			}
			return renderSQLStatement(a, *result, false)
		},
	}
}

func newSQLDeleteCmd(a *app.App) *cobra.Command {
	return &cobra.Command{
		Use:   "delete <project> <statement>",
		Short: "Delete a statement (cancels SQL if it's still pending)",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := a.API.DeleteSQLStatement(cmd.Context(), args[0], args[1]); err != nil {
				return err
			}
			a.Output.Text("Statement deleted.")
			return nil
		},
	}
}

func readSQLQuery(a *app.App, args []string) (string, error) {
	if len(args) == 2 {
		return args[1], nil
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

// pollUntilDone polls GET /sql/statements/{id} every second for up to
// 10 minutes (well past WorkerTimeout) until status leaves "pending".
// Returns the final statement; the caller decides how to render it.
func pollUntilDone(ctx context.Context, a *app.App, projectID, stmtID string) (api.SQLStatement, error) {
	deadline := time.Now().Add(10 * time.Minute)

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
		case <-time.After(time.Second):
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
		a.Output.Text("Poll: agiler sql get <project> %s", s.ID)
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
