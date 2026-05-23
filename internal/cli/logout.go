package cli

import (
	"fmt"

	"github.com/agilercloud/cli/internal/app"
	"github.com/spf13/cobra"
)

func newLogoutCmd(a *app.App) *cobra.Command {
	return &cobra.Command{
		Use:   "logout",
		Short: "Clear the stored API key",
		Long:  "Removes the API key from the persisted config file. Does not affect AGILER_API_KEY or --api-key.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := loader(a).Set("api-key", ""); err != nil {
				return fmt.Errorf("clear api-key: %w", err)
			}
			a.Output.Stderr("Logged out.")
			return nil
		},
	}
}
