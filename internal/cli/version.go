package cli

import (
	"fmt"

	"github.com/agilercloud/cli/internal/app"
	"github.com/spf13/cobra"
)

func newVersionCmd(a *app.App) *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print CLI version",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Fprintln(a.Out, a.Version)
		},
	}
}
