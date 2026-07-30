package cli

import (
	"context"
	"time"

	"github.com/agilercloud/cli/internal/api"
	"github.com/agilercloud/cli/internal/app"
	"github.com/agilercloud/cli/internal/output"
	"github.com/spf13/cobra"
)

func newRuntimesCmd(a *app.App) *cobra.Command {
	return newLookupCommand(lookupCommandOptions[api.Runtime]{
		Use:       "runtimes",
		Aliases:   []string{"runtime"},
		Short:     "List available runtimes",
		ListShort: "List all runtimes",
		GetUse:    "get <runtime-id>",
		GetShort:  "Get runtime details",
		Complete:  completeRuntimeIDs(a),
		List: func(ctx context.Context) ([]api.Runtime, error) {
			return a.API.ListRuntimes(ctx)
		},
		Get: func(ctx context.Context, runtimeID string) (*api.Runtime, error) {
			return a.API.GetRuntime(ctx, runtimeID)
		},
		RenderList: func(runtimes []api.Runtime) { renderRuntimesList(a.Output, runtimes) },
		RenderGet:  func(runtime api.Runtime) error { return renderRuntimeDetail(a.Output, runtime) },
	})
}

func renderRuntimesList(w *output.Writer, rs []api.Runtime) {
	renderTable(w, rs, "No runtimes available.",
		[]string{"ID", "DESCRIPTION", "DEPRECATED"},
		func(r api.Runtime) []string {
			deprecated := ""
			if r.DeprecatedAt != nil {
				deprecated = r.DeprecatedAt.Format(time.RFC3339)
			}
			return []string{r.Id, r.Description, deprecated}
		})
}

func renderRuntimeDetail(w *output.Writer, r api.Runtime) error {
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
	if r.DeprecatedAt != nil {
		w.Text("%s %s", w.OutColor.Dim("Deprecated: "), r.DeprecatedAt.Format(time.RFC3339))
	}
	return nil
}
