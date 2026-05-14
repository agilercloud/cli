package cli

import (
	"time"

	"github.com/agilercloud/cli/internal/api"
	"github.com/agilercloud/cli/internal/app"
	"github.com/agilercloud/cli/internal/output"
	"github.com/spf13/cobra"
)

func newRegionsCmd(a *app.App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "regions",
		Short: "List available regions",
	}

	cmd.AddCommand(&cobra.Command{
		Use:   "list",
		Short: "List all regions",
		RunE: func(cmd *cobra.Command, args []string) error {
			result, err := a.API.ListRegions(cmd.Context())
			if err != nil {
				return err
			}
			renderRegionsList(a.Output, result)
			return nil
		},
	})

	cmd.AddCommand(&cobra.Command{
		Use:   "get <region-id>",
		Short: "Get region details",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			result, err := a.API.GetRegion(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			return renderRegionDetail(a.Output, *result)
		},
	})

	return cmd
}

func renderRegionsList(w *output.Writer, rs []api.Region) {
	if w.IsStructured() {
		w.Structured(rs)
		return
	}
	if len(rs) == 0 {
		w.Text("No regions available.")
		return
	}
	rows := make([][]string, len(rs))
	for i, r := range rs {
		rows[i] = []string{r.Id, r.Description}
	}
	w.Table([]string{"ID", "DESCRIPTION"}, rows)
}

func renderRegionDetail(w *output.Writer, r api.Region) error {
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
	return nil
}
