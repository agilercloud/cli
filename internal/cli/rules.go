package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/agilercloud/cli/internal/api"
	"github.com/agilercloud/cli/internal/app"
	"github.com/spf13/cobra"
)

func newRulesCmd(a *app.App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "rules",
		Short: "Rule options and templates",
	}

	cmd.AddCommand(&cobra.Command{
		Use:   "options",
		Short: "List available rule conditions, actions, and templates",
		RunE: func(cmd *cobra.Command, args []string) error {
			var result map[string]any
			if err := a.API.DoJSON(cmd.Context(), "GET", "/v1/rules", nil, &result); err != nil {
				return err
			}
			a.Output.JSON(result)
			return nil
		},
	})

	return cmd
}

// newProjectRulesCmd is the "agiler projects rules" subcommand
func newProjectRulesCmd(a *app.App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "rules",
		Short: "Manage project rules",
	}

	cmd.AddCommand(&cobra.Command{
		Use:   "list <project>",
		Short: "List project rules",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			var result api.Project
			if err := a.API.DoJSON(cmd.Context(), "GET", fmt.Sprintf("/v1/projects/%s?expand=rules", args[0]), nil, &result); err != nil {
				return err
			}

			if a.Output.IsJSON() {
				a.Output.JSON(result.Rules)
				return nil
			}
			if len(result.Rules) == 0 {
				a.Output.Text("No rules configured.")
				return nil
			}
			a.Output.JSON(result.Rules)
			return nil
		},
	})

	cmd.AddCommand(&cobra.Command{
		Use:   "create <project> [json-file]",
		Short: "Create a project rule (reads JSON from file or stdin)",
		Args:  cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			data, err := readJSONInput(a, args, 1)
			if err != nil {
				return err
			}
			if err := a.API.DoJSON(cmd.Context(), "POST", fmt.Sprintf("/v1/projects/%s/rules", args[0]), bytes.NewReader(data), nil); err != nil {
				return err
			}
			a.Output.Text("Rule created.")
			return nil
		},
	})

	cmd.AddCommand(&cobra.Command{
		Use:   "update <project> <rule-id> [json-file]",
		Short: "Update a project rule (reads JSON from file or stdin)",
		Args:  cobra.RangeArgs(2, 3),
		RunE: func(cmd *cobra.Command, args []string) error {
			data, err := readJSONInput(a, args, 2)
			if err != nil {
				return err
			}
			var body json.RawMessage
			if err := json.Unmarshal(data, &body); err != nil {
				return fmt.Errorf("invalid JSON: %w", err)
			}
			if err := a.API.DoJSON(cmd.Context(), "PUT", fmt.Sprintf("/v1/projects/%s/rules/%s", args[0], args[1]), bytes.NewReader(data), nil); err != nil {
				return err
			}
			a.Output.Text("Rule updated.")
			return nil
		},
	})

	cmd.AddCommand(&cobra.Command{
		Use:   "delete <project> <rule-id>",
		Short: "Delete a project rule",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := a.API.DoJSON(cmd.Context(), "DELETE", fmt.Sprintf("/v1/projects/%s/rules/%s", args[0], args[1]), nil, nil); err != nil {
				return err
			}
			a.Output.Text("Rule deleted.")
			return nil
		},
	})

	return cmd
}

// readJSONInput reads from args[fileIdx] if present, else from stdin.
func readJSONInput(a *app.App, args []string, fileIdx int) ([]byte, error) {
	if len(args) > fileIdx {
		data, err := os.ReadFile(args[fileIdx])
		if err != nil {
			return nil, fmt.Errorf("read rule data: %w", err)
		}
		return data, nil
	}
	data, err := io.ReadAll(a.In)
	if err != nil {
		return nil, fmt.Errorf("read rule data: %w", err)
	}
	return data, nil
}
