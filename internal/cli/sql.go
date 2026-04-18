package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/agilercloud/cli/internal/app"
	"github.com/spf13/cobra"
)

func newSQLCmd(a *app.App) *cobra.Command {
	return &cobra.Command{
		Use:   "sql <project> [query]",
		Short: "Execute SQL against a project database",
		Long:  "Execute a SQL query. Provide the query as an argument, or pipe it via stdin.",
		Args:  cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			var query string
			if len(args) == 2 {
				query = args[1]
			} else {
				data, err := io.ReadAll(a.In)
				if err != nil {
					return fmt.Errorf("read stdin: %w", err)
				}
				query = strings.TrimSpace(string(data))
			}

			if query == "" {
				return fmt.Errorf("no query provided")
			}

			body := map[string]any{"query": query}
			data, _ := json.Marshal(body)

			var result any
			if err := a.API.DoJSON(cmd.Context(), "POST", fmt.Sprintf("/v1/projects/%s/db/sql", args[0]), bytes.NewReader(data), &result); err != nil {
				return err
			}
			a.Output.JSON(result)
			return nil
		},
	}
}
