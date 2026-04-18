package cli

import (
	"fmt"

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
			var result []api.Region
			if err := a.API.DoJSON(cmd.Context(), "GET", "/v1/regions", nil, &result); err != nil {
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
			var result api.Region
			if err := a.API.DoJSON(cmd.Context(), "GET", fmt.Sprintf("/v1/regions/%s", args[0]), nil, &result); err != nil {
				return err
			}
			renderRegionDetail(a.Output, result)
			return nil
		},
	})

	return cmd
}

func renderRegionsList(w *output.Writer, rs []api.Region) {
	if w.IsJSON() {
		w.JSON(rs)
		return
	}
	if len(rs) == 0 {
		w.Text("No regions available.")
		return
	}
	rows := make([][]string, len(rs))
	for i, r := range rs {
		rows[i] = []string{r.ID, r.Description}
	}
	w.Table([]string{"ID", "DESCRIPTION"}, rows)
}

func renderRegionDetail(w *output.Writer, r api.Region) {
	if w.IsJSON() {
		w.JSON(r)
		return
	}
	w.Text("ID:          %s", r.ID)
	w.Text("Description: %s", r.Description)
	w.Text("Created:     %s", r.CreatedAt)
	w.Text("Updated:     %s", r.UpdatedAt)
}
