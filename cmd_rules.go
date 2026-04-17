package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"
)

// rulesCmd is the top-level "agiler rules" command for rule options
var rulesCmd = &cobra.Command{
	Use:   "rules",
	Short: "Rule options and templates",
}

func init() {
	rulesCmd.AddCommand(rulesOptionsCmd)
}

var rulesOptionsCmd = &cobra.Command{
	Use:   "options",
	Short: "List available rule conditions, actions, and templates",
	RunE: func(cmd *cobra.Command, args []string) error {
		var result map[string]any
		if err := apiClient.DoJSON("GET", "/v1/rules", nil, &result); err != nil {
			return err
		}

		// always output JSON for this complex structure
		printJSON(result)
		return nil
	},
}

// projectRulesCmd is the "agiler projects rules" subcommand
var projectRulesCmd = &cobra.Command{
	Use:   "rules",
	Short: "Manage project rules",
}

func init() {
	projectRulesCmd.AddCommand(projectRulesListCmd)
	projectRulesCmd.AddCommand(projectRulesCreateCmd)
	projectRulesCmd.AddCommand(projectRulesUpdateCmd)
	projectRulesCmd.AddCommand(projectRulesDeleteCmd)
}

var projectRulesListCmd = &cobra.Command{
	Use:   "list <project>",
	Short: "List project rules",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		var result map[string]any
		if err := apiClient.DoJSON("GET", fmt.Sprintf("/v1/projects/%s?expand=rules", args[0]), nil, &result); err != nil {
			return err
		}

		rules, _ := result["rules"].([]any)

		if outputJSON {
			printJSON(rules)
			return nil
		}

		if len(rules) == 0 {
			fmt.Println("No rules configured.")
			return nil
		}

		// rules are complex structures, default to JSON
		printJSON(rules)
		return nil
	},
}

var projectRulesCreateCmd = &cobra.Command{
	Use:   "create <project> [json-file]",
	Short: "Create a project rule (reads JSON from file or stdin)",
	Args:  cobra.RangeArgs(1, 2),
	RunE: func(cmd *cobra.Command, args []string) error {
		var data []byte
		var err error

		if len(args) == 2 {
			data, err = os.ReadFile(args[1])
		} else {
			data, err = io.ReadAll(os.Stdin)
		}
		if err != nil {
			return fmt.Errorf("read rule data: %w", err)
		}

		if err := apiClient.DoJSON("POST", fmt.Sprintf("/v1/projects/%s/rules", args[0]), bytes.NewReader(data), nil); err != nil {
			return err
		}

		fmt.Println("Rule created.")
		return nil
	},
}

var projectRulesUpdateCmd = &cobra.Command{
	Use:   "update <project> <rule-id> [json-file]",
	Short: "Update a project rule (reads JSON from file or stdin)",
	Args:  cobra.RangeArgs(2, 3),
	RunE: func(cmd *cobra.Command, args []string) error {
		var data []byte
		var err error

		if len(args) == 3 {
			data, err = os.ReadFile(args[2])
		} else {
			data, err = io.ReadAll(os.Stdin)
		}
		if err != nil {
			return fmt.Errorf("read rule data: %w", err)
		}

		// validate JSON
		var body json.RawMessage
		if err := json.Unmarshal(data, &body); err != nil {
			return fmt.Errorf("invalid JSON: %w", err)
		}

		if err := apiClient.DoJSON("PUT", fmt.Sprintf("/v1/projects/%s/rules/%s", args[0], args[1]), bytes.NewReader(data), nil); err != nil {
			return err
		}

		fmt.Println("Rule updated.")
		return nil
	},
}

var projectRulesDeleteCmd = &cobra.Command{
	Use:   "delete <project> <rule-id>",
	Short: "Delete a project rule",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := apiClient.DoJSON("DELETE", fmt.Sprintf("/v1/projects/%s/rules/%s", args[0], args[1]), nil, nil); err != nil {
			return err
		}

		fmt.Println("Rule deleted.")
		return nil
	},
}
