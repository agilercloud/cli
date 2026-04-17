package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var (
	Version = "dev"

	flagConfig  string
	flagAPIKey  string
	flagAPIBase string

	apiClient *Client
)

func main() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

var rootCmd = &cobra.Command{
	Use:   "agiler",
	Short: "Agiler CLI — manage your Agiler projects from the terminal",
	Long:  "Agiler CLI allows you to manage projects, files, backups, and more using an API key.",
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		backgroundUpdateCheck(cmd.Name())

		// skip client setup for commands that don't need it
		switch cmd.Name() {
		case "version", "help", "upgrade":
			return
		}
		if cmd.Parent() != nil && cmd.Parent().Name() == "config" {
			return
		}

		cfg, err := loadConfig()
		if err != nil {
			fatalf("Error loading config: %v", err)
		}

		// flag overrides
		if flagAPIKey != "" {
			cfg.APIKey = flagAPIKey
		}
		if flagAPIBase != "" {
			cfg.APIBase = flagAPIBase
		}

		apiClient = NewClient(cfg.APIBase, cfg.APIKey)
	},
	SilenceUsage: true,
}

func init() {
	rootCmd.PersistentFlags().StringVarP(&flagConfig, "config", "c", "", "Config file path")
	rootCmd.PersistentFlags().StringVar(&flagAPIKey, "api-key", "", "API key (overrides config and AGILER_API_KEY)")
	rootCmd.PersistentFlags().StringVar(&flagAPIBase, "api-base", "", "API base URL (overrides config and AGILER_API_BASE)")
	rootCmd.PersistentFlags().BoolVar(&outputJSON, "json", false, "Output raw JSON")
	rootCmd.PersistentFlags().BoolVarP(&outputQuiet, "quiet", "q", false, "Minimal output (IDs only)")

	rootCmd.AddCommand(statusCmd)
	rootCmd.AddCommand(configCmd)
	rootCmd.AddCommand(projectsCmd)
	rootCmd.AddCommand(runtimesCmd)
	rootCmd.AddCommand(regionsCmd)
	rootCmd.AddCommand(rulesCmd)
	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(upgradeCmd)
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print CLI version",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println(Version)
	},
}

// configCmd handles config get/set/path
var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage CLI configuration",
}

var configSetCmd = &cobra.Command{
	Use:   "set <key> <value>",
	Short: "Set a config value",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := configSet(args[0], args[1]); err != nil {
			return err
		}
		fmt.Printf("Set %s\n", args[0])
		return nil
	},
}

var configGetCmd = &cobra.Command{
	Use:   "get <key>",
	Short: "Get a config value",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		v, err := configGet(args[0])
		if err != nil {
			return err
		}
		fmt.Println(v)
		return nil
	},
}

var configPathCmd = &cobra.Command{
	Use:   "path",
	Short: "Print config file path",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println(configPath())
	},
}

func init() {
	configCmd.AddCommand(configSetCmd)
	configCmd.AddCommand(configGetCmd)
	configCmd.AddCommand(configPathCmd)
}
