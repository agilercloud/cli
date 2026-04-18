package cli

import (
	"fmt"

	"github.com/agilercloud/cli/internal/api"
	"github.com/agilercloud/cli/internal/app"
	"github.com/agilercloud/cli/internal/output"
	"github.com/spf13/cobra"
)

func newRuntimesCmd(a *app.App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "runtimes",
		Short: "List available runtimes",
	}

	cmd.AddCommand(&cobra.Command{
		Use:   "list",
		Short: "List all runtimes",
		RunE: func(cmd *cobra.Command, args []string) error {
			var result []api.Runtime
			if err := a.API.DoJSON(cmd.Context(), "GET", "/v1/runtimes", nil, &result); err != nil {
				return err
			}
			renderRuntimesList(a.Output, result)
			return nil
		},
	})

	cmd.AddCommand(&cobra.Command{
		Use:   "get <runtime-id>",
		Short: "Get runtime details",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			var result api.Runtime
			if err := a.API.DoJSON(cmd.Context(), "GET", fmt.Sprintf("/v1/runtimes/%s", args[0]), nil, &result); err != nil {
				return err
			}
			renderRuntimeDetail(a.Output, result)
			return nil
		},
	})

	return cmd
}

func renderRuntimesList(w *output.Writer, rs []api.Runtime) {
	if w.IsJSON() {
		w.JSON(rs)
		return
	}
	if len(rs) == 0 {
		w.Text("No runtimes available.")
		return
	}
	rows := make([][]string, len(rs))
	for i, r := range rs {
		deprecated := ""
		if r.DeprecatedAt != nil {
			deprecated = *r.DeprecatedAt
		}
		rows[i] = []string{r.ID, r.Description, deprecated}
	}
	w.Table([]string{"ID", "DESCRIPTION", "DEPRECATED"}, rows)
}

func renderRuntimeDetail(w *output.Writer, r api.Runtime) {
	if w.IsJSON() {
		w.JSON(r)
		return
	}
	w.Text("ID:          %s", r.ID)
	w.Text("Description: %s", r.Description)
	w.Text("Created:     %s", r.CreatedAt)
	w.Text("Updated:     %s", r.UpdatedAt)
	if r.DeprecatedAt != nil {
		w.Text("Deprecated:  %s", *r.DeprecatedAt)
	}
}
