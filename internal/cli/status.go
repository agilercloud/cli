package cli

import (
	"fmt"

	"github.com/agilercloud/cli/internal/app"
	"github.com/spf13/cobra"
)

func newStatusCmd(a *app.App) *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Check API status",
		RunE: func(cmd *cobra.Command, args []string) error {
			result, err := a.API.GetStatus(cmd.Context())
			if err != nil {
				return fmt.Errorf("status check failed: %w", err)
			}

			if a.Output.IsTabular() {
				return tabularUnsupportedErr(a.Output)
			}
			if a.Output.IsStructured() {
				a.Output.Structured(result)
				return nil
			}
			status := result.Status
			if status == "ok" {
				status = a.Output.OutColor.Green(status)
			} else {
				status = a.Output.OutColor.Yellow(status)
			}
			a.Output.Text("status: %s", status)
			return nil
		},
	}
}
