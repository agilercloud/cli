package cli

import (
	"fmt"

	"github.com/agilercloud/cli/internal/api"
	"github.com/agilercloud/cli/internal/app"
	"github.com/spf13/cobra"
)

func newStatusCmd(a *app.App) *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Check API status",
		RunE: func(cmd *cobra.Command, args []string) error {
			var result api.Status
			if err := a.API.DoJSON(cmd.Context(), "GET", "/status", nil, &result); err != nil {
				return fmt.Errorf("status check failed: %w", err)
			}

			if a.Output.IsJSON() {
				a.Output.JSON(result)
				return nil
			}
			a.Output.Text("status: %s", result.Status)
			return nil
		},
	}
}
