package cli

import (
	"context"
	"time"

	"github.com/agilercloud/cli/internal/api"
	"github.com/agilercloud/cli/internal/app"
	"github.com/agilercloud/cli/internal/output"
	"github.com/spf13/cobra"
)

func newRegionsCmd(a *app.App) *cobra.Command {
	return newLookupCommand(lookupCommandOptions[api.Region]{
		Use:       "regions",
		Aliases:   []string{"region"},
		Short:     "List available regions",
		ListShort: "List all regions",
		GetUse:    "get <region-id>",
		GetShort:  "Get region details",
		Complete:  completeRegionIDs(a),
		List: func(ctx context.Context) ([]api.Region, error) {
			return a.API.ListRegions(ctx)
		},
		Get: func(ctx context.Context, regionID string) (*api.Region, error) {
			return a.API.GetRegion(ctx, regionID)
		},
		RenderList: func(regions []api.Region) { renderRegionsList(a.Output, regions) },
		RenderGet:  func(region api.Region) error { return renderRegionDetail(a.Output, region) },
	})
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
	w.Text("%s %s", w.OutColor.Dim("ID:         "), r.Id)
	w.Text("%s %s", w.OutColor.Dim("Description:"), r.Description)
	w.Text("%s %s", w.OutColor.Dim("Created:    "), r.CreatedAt.Format(time.RFC3339))
	w.Text("%s %s", w.OutColor.Dim("Updated:    "), r.UpdatedAt.Format(time.RFC3339))
	return nil
}
