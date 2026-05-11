package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/agilercloud/cli/internal/api"
	"github.com/agilercloud/cli/internal/app"
	"github.com/spf13/cobra"
)

// newSQLCmd builds the `agiler sql ...` subcommand tree.
//
//	agiler sql submit <project> [query] [--read-only] [--timeout=SECONDS] [--async]
//	agiler sql history <project> [--limit=N]
//	agiler sql get <project> <statement>
//	agiler sql delete <project> <statement>
//
// `submit` is also exposed as the default `agiler sql <project> [query]` for
// backwards-compatibility with the prior single-command shape.
func newSQLCmd(a *app.App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "sql",
		Short: "Execute SQL against a project database and inspect prior runs",
		Long:  "Submit, list, and inspect SQL executions on a project database.",
	}
	cmd.AddCommand(newSQLSubmitCmd(a))
	cmd.AddCommand(newSQLHistoryCmd(a))
	cmd.AddCommand(newSQLGetCmd(a))
	cmd.AddCommand(newSQLDeleteCmd(a))
	return cmd
}

func newSQLSubmitCmd(a *app.App) *cobra.Command {
	var readOnly bool
	var timeout int
	var async bool

	cmd := &cobra.Command{
		Use:     "submit <project> [query]",
		Aliases: []string{"exec", "run"},
		Short:   "Execute a SQL statement against a project database",
		Long:    "Execute SQL. Provide the statement as an argument, or pipe it via stdin.",
		Args:    cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			query, err := readSQLQuery(a, args)
			if err != nil {
				return err
			}

			body := map[string]any{
				"sql":       query,
				"read_only": readOnly,
			}
			if timeout > 0 {
				body["timeout"] = timeout
			}
			data, _ := json.Marshal(body)

			path := fmt.Sprintf("/v1/projects/%s/sql/statements", args[0])
			headers := map[string]string{}
			if async {
				headers["Prefer"] = "respond-async"
			}
			resp, err := a.API.DoRaw(cmd.Context(), http.MethodPost, path, "application/json", headers, bytes.NewReader(data))
			if err != nil {
				return err
			}
			defer func() { _ = resp.Body.Close() }()

			var result api.SQLStatement
			if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
				return fmt.Errorf("decode response: %w", err)
			}
			return renderSQLStatement(a, result, async)
		},
	}
	cmd.Flags().BoolVar(&readOnly, "read-only", false, "Wrap execution in a read-only transaction")
	cmd.Flags().IntVar(&timeout, "timeout", 0, "Per-statement timeout in seconds (server-side)")
	cmd.Flags().BoolVar(&async, "async", false, "Send Prefer: respond-async; returns 202 with status: pending")
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
			path := fmt.Sprintf("/v1/projects/%s/sql/statements", args[0])
			if limit > 0 {
				path = fmt.Sprintf("%s?limit=%d", path, limit)
			}
			var result []api.SQLStatement
			if err := a.API.DoJSON(cmd.Context(), http.MethodGet, path, nil, &result); err != nil {
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
			path := fmt.Sprintf("/v1/projects/%s/sql/statements/%s", args[0], args[1])
			var result api.SQLStatement
			if err := a.API.DoJSON(cmd.Context(), http.MethodGet, path, nil, &result); err != nil {
				return err
			}
			return renderSQLStatement(a, result, false)
		},
	}
}

func newSQLDeleteCmd(a *app.App) *cobra.Command {
	return &cobra.Command{
		Use:   "delete <project> <statement>",
		Short: "Cancel a pending execution or remove a history entry",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			path := fmt.Sprintf("/v1/projects/%s/sql/statements/%s", args[0], args[1])
			if err := a.API.DoJSON(cmd.Context(), http.MethodDelete, path, nil, nil); err != nil {
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
		a.Output.Text("Poll: agiler sql get %s %s", projectIDFromContext(), s.ID)
	}
	return nil
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

// projectIDFromContext is a placeholder for the optional "show next-step
// hint" message after an async submit. We can't recover the project id
// from the API response (the resource is identified by URL), so we leave
// a stub the user can replace with their project name. Doing nothing here
// at all (or printing the Location header) is also fine; this is purely a
// UX nicety.
func projectIDFromContext() string {
	return "<project>"
}

// formatStatementTime is a small helper used by callers that need to print
// time.Time values in the same format the API uses on the wire. The CLI's
// renderers consume strings directly, so this is rarely needed today; it
// keeps the door open for the SPA-side renderer adopting the same shape.
func formatStatementTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339Nano)
}
