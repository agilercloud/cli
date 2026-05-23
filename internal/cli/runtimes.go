package cli

import (
	"time"

	"github.com/agilercloud/cli/internal/api"
	"github.com/agilercloud/cli/internal/app"
	"github.com/agilercloud/cli/internal/output"
	"github.com/spf13/cobra"
)

func newRuntimesCmd(a *app.App) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "runtimes",
		Aliases: []string{"runtime"},
		Short:   "List available runtimes",
	}

	cmd.AddCommand(&cobra.Command{
		Use:   "list",
		Short: "List all runtimes",
		RunE: func(cmd *cobra.Command, args []string) error {
			result, err := a.API.ListRuntimes(cmd.Context())
			if err != nil {
				return err
			}
			renderRuntimesList(a.Output, result)
			return nil
		},
	})

	cmd.AddCommand(&cobra.Command{
		Use:               "get <runtime-id>",
		Short:             "Get runtime details",
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: completeRuntimeIDs(a),
		RunE: func(cmd *cobra.Command, args []string) error {
			result, err := a.API.GetRuntime(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			return renderRuntimeDetail(a.Output, *result)
		},
	})

	return cmd
}

func renderRuntimesList(w *output.Writer, rs []api.Runtime) {
	if w.IsStructured() {
		w.Structured(rs)
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
			deprecated = r.DeprecatedAt.Format(time.RFC3339)
		}
		rows[i] = []string{r.Id, r.Description, deprecated}
	}
	w.Table([]string{"ID", "DESCRIPTION", "DEPRECATED"}, rows)
}

func renderRuntimeDetail(w *output.Writer, r api.Runtime) error {
	if w.IsTabular() {
		return tabularUnsupportedErr(w)
	}
	if w.IsStructured() {
		w.Structured(r)
		return nil
	}
	w.Text("ID:          %s", r.Id)
	w.Text("Description: %s", r.Description)
	w.Text("Created:     %s", r.CreatedAt.Format(time.RFC3339))
	w.Text("Updated:     %s", r.UpdatedAt.Format(time.RFC3339))
	if r.DeprecatedAt != nil {
		w.Text("Deprecated:  %s", r.DeprecatedAt.Format(time.RFC3339))
	}
	return nil
}
