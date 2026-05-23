package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/agilercloud/cli/internal/api"
	"github.com/agilercloud/cli/internal/app"
	"github.com/spf13/cobra"
)

// newRulesCmd builds the top-level `agiler rules ...` tree. CRUD on a
// project's rules lives directly under `rules`; the platform-wide
// condition/action/template catalog lives at `rules templates options`.
func newRulesCmd(a *app.App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "rules",
		Short: "Manage project rules",
	}

	cmd.AddCommand(&cobra.Command{
		Use:   "list",
		Short: "List project rules",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			projectID, err := requireProjectID(a)
			if err != nil {
				return err
			}
			result, err := a.API.ListProjectRules(cmd.Context(), projectID)
			if err != nil {
				return err
			}

			if a.Output.IsTabular() {
				return tabularUnsupportedErr(a.Output)
			}
			if len(result) == 0 && !a.Output.IsStructured() {
				a.Output.Text("No rules configured.")
				return nil
			}
			a.Output.JSON(result)
			return nil
		},
	})

	cmd.AddCommand(&cobra.Command{
		Use:   "create [json-file]",
		Short: "Create a project rule (reads JSON from file or stdin)",
		Args:  cobra.RangeArgs(0, 1),
		RunE: func(cmd *cobra.Command, args []string) error {
			projectID, err := requireProjectID(a)
			if err != nil {
				return err
			}
			data, err := readJSONInput(a, args, 0)
			if err != nil {
				return err
			}
			var in api.CreateRuleInput
			if err := json.Unmarshal(data, &in); err != nil {
				return fmt.Errorf("invalid rule JSON: %w", err)
			}
			if _, err := a.API.CreateRule(cmd.Context(), projectID, in); err != nil {
				return err
			}
			a.Output.Text("Rule created.")
			return nil
		},
	})

	cmd.AddCommand(&cobra.Command{
		Use:   "update <rule-id> [json-file]",
		Short: "Update a project rule (reads JSON from file or stdin)",
		Args:  cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			projectID, err := requireProjectID(a)
			if err != nil {
				return err
			}
			data, err := readJSONInput(a, args, 1)
			if err != nil {
				return err
			}
			var in api.UpdateRuleInput
			if err := json.Unmarshal(data, &in); err != nil {
				return fmt.Errorf("invalid rule JSON: %w", err)
			}
			r, err := a.API.UpdateRule(cmd.Context(), projectID, args[0], in)
			if err != nil {
				return err
			}
			if a.Output.IsStructured() {
				a.Output.Structured(r)
			} else {
				a.Output.Text("Rule %s updated.", r.Name)
			}
			return nil
		},
	})

	cmd.AddCommand(&cobra.Command{
		Use:   "delete <rule-id>",
		Short: "Delete a project rule",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			projectID, err := requireProjectID(a)
			if err != nil {
				return err
			}
			if err := a.API.DeleteRule(cmd.Context(), projectID, args[0]); err != nil {
				return err
			}
			a.Output.Text("Rule deleted.")
			return nil
		},
	})

	cmd.AddCommand(newRuleTemplatesCmd(a))

	return cmd
}

// newRuleTemplatesCmd exposes the platform's rule condition/action/template
// catalog under `rules templates options`. These are reference data, not
// per-project.
func newRuleTemplatesCmd(a *app.App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "templates",
		Short: "Browse the platform rule catalog (conditions, actions, templates)",
	}

	cmd.AddCommand(&cobra.Command{
		Use:   "options",
		Short: "List available rule conditions, actions, and templates",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			result, err := a.API.ListRuleOptions(cmd.Context())
			if err != nil {
				return err
			}
			if a.Output.IsTabular() {
				return tabularUnsupportedErr(a.Output)
			}
			a.Output.JSON(result)
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
