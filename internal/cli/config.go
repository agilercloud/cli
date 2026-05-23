package cli

import (
	"fmt"
	"io"
	"strings"

	"github.com/agilercloud/cli/internal/app"
	"github.com/agilercloud/cli/internal/config"
	"github.com/spf13/cobra"
)

// loader returns a.ConfigLoader or a fresh OS loader if not yet initialized.
// The config subcommands run with PersistentPreRunE skipping API setup, so
// ConfigLoader may be nil here.
func loader(a *app.App) config.Loader {
	if a.ConfigLoader != nil {
		return a.ConfigLoader
	}
	return config.NewOSLoader(config.Options{FlagConfig: a.FlagConfig})
}

func newConfigCmd(a *app.App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Manage CLI configuration",
	}

	cmd.AddCommand(&cobra.Command{
		Use:   "set <key> <value>",
		Short: "Set a config value (use '-' as <value> to read from stdin)",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			value := args[1]
			if value == "-" {
				data, err := io.ReadAll(a.In)
				if err != nil {
					return fmt.Errorf("read value from stdin: %w", err)
				}
				value = strings.TrimRight(string(data), " \t\r\n")
			}
			if err := loader(a).Set(args[0], value); err != nil {
				return err
			}
			_, _ = fmt.Fprintf(a.Out, "Set %s\n", args[0])
			return nil
		},
	})

	cmd.AddCommand(&cobra.Command{
		Use:   "get <key>",
		Short: "Get a config value",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			v, err := loader(a).Get(args[0])
			if err != nil {
				return err
			}
			_, _ = fmt.Fprintln(a.Out, v)
			return nil
		},
	})

	cmd.AddCommand(&cobra.Command{
		Use:   "path",
		Short: "Print config file path",
		Run: func(cmd *cobra.Command, args []string) {
			_, _ = fmt.Fprintln(a.Out, loader(a).Path())
		},
	})

	return cmd
}
