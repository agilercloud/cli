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
			if err := initOutput(a); err != nil {
				return err
			}

			updatecheck.Background(updatecheck.Options{
				CmdName:     cmd.Name(),
				Version:     a.Version,
				OutputMuted: a.Output.Format != output.FormatText || a.Output.Quiet,
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
			case "version", "help", "upgrade", "login", "logout":
				return nil
			}
			if cmd.Parent() != nil && cmd.Parent().Name() == "config" {
				return nil
			}

			return ensureAPI(a)
		},
		SilenceUsage: true,
	}

	root.PersistentFlags().StringVarP(&a.FlagConfig, "config", "c", "", "Config file path")
	root.PersistentFlags().StringVar(&a.FlagAPIKey, "api-key", "", "API key (overrides config and AGILER_API_KEY)")
	root.PersistentFlags().StringVar(&a.FlagAPIBase, "api-base", "", "API base URL (overrides config and AGILER_API_BASE)")
	root.PersistentFlags().StringVar(&a.FlagWorkspaceID, "workspace", "", "Workspace ID (overrides config and AGILER_WORKSPACE_ID)")
	root.PersistentFlags().StringVarP(&a.FlagProjectID, "project", "p", "", "Project ID for project-scoped commands (overrides config and AGILER_PROJECT_ID)")
	root.PersistentFlags().StringVar(&a.OutputFormat, "format", "", "Output format: text|json|yaml|csv|tsv (default text)")
	root.PersistentFlags().BoolVarP(&a.OutputQuiet, "quiet", "q", false, "Minimal output (IDs only)")

	_ = root.RegisterFlagCompletionFunc("project", completeProjectIDs(a))
	_ = root.RegisterFlagCompletionFunc("workspace", completeWorkspaceIDs(a))

	root.AddGroup(
		&cobra.Group{ID: "project-ops", Title: "Project operations:"},
		&cobra.Group{ID: "resources", Title: "Resources:"},
		&cobra.Group{ID: "reference", Title: "Reference:"},
		&cobra.Group{ID: "account", Title: "Account:"},
		&cobra.Group{ID: "maintenance", Title: "Maintenance:"},
	)

	add := func(groupID string, c *cobra.Command) {
		c.GroupID = groupID
		root.AddCommand(c)
	}

	add("project-ops", newLogsCmd(a))
	add("project-ops", newSQLCmd(a))
	add("project-ops", newFilesCmd(a))
	add("project-ops", newBackupsCmd(a))
	add("project-ops", newVariablesCmd(a))
	add("project-ops", newDomainsCmd(a))
	add("project-ops", newRulesCmd(a))
	add("project-ops", newUsageCmd(a))

	add("resources", newProjectsCmd(a))
	add("resources", newWorkspacesCmd(a))

	add("reference", newRegionsCmd(a))
	add("reference", newRuntimesCmd(a))

	add("account", newLoginCmd(a))
	add("account", newLogoutCmd(a))
	add("account", newWhoamiCmd(a))
	add("account", newBillingCmd(a))
	add("account", newNotificationsCmd(a))
	add("account", newConfigCmd(a))

	add("maintenance", newStatusCmd(a))
	add("maintenance", newUpgradeCmd(a))
	add("maintenance", newVersionCmd(a))

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

// ensureAPI loads config and constructs a.API if it hasn't been done yet.
// PersistentPreRunE calls it for the normal command path; completion
// helpers call it on demand because Cobra's __complete entry point does
// not invoke PersistentPreRunE.
func ensureAPI(a *app.App) error {
	if a.API != nil {
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
	if a.FlagWorkspaceID != "" {
		cfg.WorkspaceID = a.FlagWorkspaceID
	}
	if a.FlagProjectID != "" {
		cfg.ProjectID = a.FlagProjectID
	}
	a.Config = cfg
	a.API = api.NewClient(cfg.APIBase, cfg.APIKey)
	return nil
}

func initOutput(a *app.App) error {
	format := output.FormatText
	if a.OutputFormat != "" {
		f, err := output.ParseFormat(a.OutputFormat)
		if err != nil {
			return err
		}
		format = f
	}
	a.Output = output.New(format, a.OutputQuiet, a.Out, a.Err)
	return nil
}
