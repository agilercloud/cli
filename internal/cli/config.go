package cli

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/BurntSushi/toml"
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

	cmd.AddCommand(newConfigShowCmd(a))

	return cmd
}

type configEntry struct {
	Key    string `json:"key"`
	Value  string `json:"value"`
	Source string `json:"source"`
}

func newConfigShowCmd(a *app.App) *cobra.Command {
	return &cobra.Command{
		Use:   "show",
		Short: "Show the resolved config and where each value came from",
		Long: "Prints every config key, its effective value, and which source provided it " +
			"(flag, env var, file, default, or unset). Sensitive values like api_key are redacted.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			entries := resolveConfig(a)
			w := a.Output
			if w.IsStructured() {
				w.Structured(entries)
				return nil
			}
			if w.IsTabular() {
				rows := make([][]string, len(entries))
				for i, e := range entries {
					rows[i] = []string{e.Key, e.Value, e.Source}
				}
				w.Table([]string{"KEY", "VALUE", "SOURCE"}, rows)
				return nil
			}
			if w.IsQuiet() {
				for _, e := range entries {
					w.Text("%s=%s", e.Key, e.Value)
				}
				return nil
			}
			width := 0
			for _, e := range entries {
				if len(e.Key) > width {
					width = len(e.Key)
				}
			}
			for _, e := range entries {
				value := e.Value
				if value == "" {
					value = "(unset)"
				}
				w.Text("%-*s = %-30s (%s)", width, e.Key, value, e.Source)
			}
			return nil
		},
	}
}

// resolveConfig walks the four configured keys and reports the effective
// value plus where it came from. File presence is detected by re-reading
// the TOML directly so env-var-only values aren't misattributed.
func resolveConfig(a *app.App) []configEntry {
	cfg, _ := loader(a).Load()
	path := loader(a).Path()

	fileCfg := &config.Config{}
	if data, err := os.ReadFile(path); err == nil {
		_ = toml.Unmarshal(data, fileCfg)
	}

	keys := []struct {
		key      string
		flag     string
		flagName string
		env      string
		fileVal  string
		loaded   string
		isSecret bool
		hasDef   bool
		defVal   string
	}{
		{"api_key", a.FlagAPIKey, "--api-key", "AGILER_API_KEY", fileCfg.APIKey, cfg.APIKey, true, false, ""},
		{"api_base", a.FlagAPIBase, "--api-base", "AGILER_API_BASE", fileCfg.APIBase, cfg.APIBase, false, true, config.DefaultAPIBase},
		{"workspace_id", a.FlagWorkspaceID, "--workspace", "AGILER_WORKSPACE_ID", fileCfg.WorkspaceID, cfg.WorkspaceID, false, false, ""},
		{"project_id", a.FlagProjectID, "--project", "AGILER_PROJECT_ID", fileCfg.ProjectID, cfg.ProjectID, false, false, ""},
	}

	entries := make([]configEntry, 0, len(keys))
	for _, k := range keys {
		value, source := resolveSource(k.flag, k.flagName, k.env, k.fileVal, k.loaded, path, k.hasDef, k.defVal)
		if k.isSecret {
			value = redact(value)
		}
		entries = append(entries, configEntry{Key: k.key, Value: value, Source: source})
	}
	return entries
}

func resolveSource(flag, flagName, envName, fileVal, loaded, path string, hasDef bool, defVal string) (string, string) {
	if flag != "" {
		return flag, fmt.Sprintf("flag (%s)", flagName)
	}
	if v := os.Getenv(envName); v != "" {
		return v, fmt.Sprintf("env (%s)", envName)
	}
	if fileVal != "" && fileVal == loaded {
		return loaded, fmt.Sprintf("file (%s)", path)
	}
	if hasDef && loaded == defVal {
		return loaded, "default"
	}
	return "", "unset"
}

func redact(v string) string {
	if v == "" {
		return ""
	}
	if len(v) < 12 {
		return "***"
	}
	return v[:4] + "***" + v[len(v)-4:]
}
