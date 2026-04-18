package cli

import (
	"context"
	"fmt"

	"github.com/agilercloud/cli/internal/api"
	"github.com/agilercloud/cli/internal/app"
	"github.com/agilercloud/cli/internal/config"
	"github.com/agilercloud/cli/internal/output"
	"github.com/agilercloud/cli/internal/selfupdate"
	"github.com/agilercloud/cli/internal/updatecheck"
	"github.com/spf13/cobra"
)

// NewRootCmd builds the agiler command tree. The returned *cobra.Command
// has its PersistentPreRunE wired to initialize a.Output and a.API based
// on the parsed flags + config.
func NewRootCmd(a *app.App) *cobra.Command {
	root := &cobra.Command{
		Use:   "agiler",
		Short: "Agiler CLI — manage your Agiler projects from the terminal",
		Long:  "Agiler CLI allows you to manage projects, files, backups, and more using an API key.",
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			initOutput(a)

			updatecheck.Background(updatecheck.Options{
				CmdName:     cmd.Name(),
				Version:     a.Version,
				OutputMuted: a.OutputJSON || a.OutputQuiet,
				Fetch: func(ctx context.Context) (string, error) {
					rel, err := selfupdate.FetchRelease(ctx, "")
					if err != nil {
						return "", err
					}
					return rel.TagName, nil
				},
			})

			// skip API setup for commands that don't need it
			switch cmd.Name() {
			case "version", "help", "upgrade":
				return nil
			}
			if cmd.Parent() != nil && cmd.Parent().Name() == "config" {
				return nil
			}

			if a.ConfigLoader == nil {
				a.ConfigLoader = config.NewOSLoader(config.Options{FlagConfig: a.FlagConfig})
			}
			cfg, err := a.ConfigLoader.Load()
			if err != nil {
				return fmt.Errorf("load config: %w", err)
			}
			if a.FlagAPIKey != "" {
				cfg.APIKey = a.FlagAPIKey
			}
			if a.FlagAPIBase != "" {
				cfg.APIBase = a.FlagAPIBase
			}
			a.Config = cfg
			a.API = api.NewClient(cfg.APIBase, cfg.APIKey)
			return nil
		},
		SilenceUsage: true,
	}

	root.PersistentFlags().StringVarP(&a.FlagConfig, "config", "c", "", "Config file path")
	root.PersistentFlags().StringVar(&a.FlagAPIKey, "api-key", "", "API key (overrides config and AGILER_API_KEY)")
	root.PersistentFlags().StringVar(&a.FlagAPIBase, "api-base", "", "API base URL (overrides config and AGILER_API_BASE)")
	root.PersistentFlags().BoolVar(&a.OutputJSON, "json", false, "Output raw JSON")
	root.PersistentFlags().BoolVarP(&a.OutputQuiet, "quiet", "q", false, "Minimal output (IDs only)")

	root.AddCommand(newStatusCmd(a))
	root.AddCommand(newConfigCmd(a))
	root.AddCommand(newProjectsCmd(a))
	root.AddCommand(newRuntimesCmd(a))
	root.AddCommand(newRegionsCmd(a))
	root.AddCommand(newRulesCmd(a))
	root.AddCommand(newVersionCmd(a))
	root.AddCommand(newUpgradeCmd(a))

	return root
}

// Run executes the CLI and returns a process exit code.
func Run(a *app.App, ctx context.Context, args []string) int {
	root := NewRootCmd(a)
	root.SetArgs(args)
	root.SetIn(a.In)
	root.SetOut(a.Out)
	root.SetErr(a.Err)
	if err := root.ExecuteContext(ctx); err != nil {
		return 1
	}
	return 0
}

func initOutput(a *app.App) {
	mode := output.ModeText
	switch {
	case a.OutputJSON:
		mode = output.ModeJSON
	case a.OutputQuiet:
		mode = output.ModeQuiet
	}
	a.Output = output.New(mode, a.Out, a.Err)
}
