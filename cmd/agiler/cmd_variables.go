package main

import (
	"bytes"
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
)

var variablesCmd = &cobra.Command{
	Use:   "variables",
	Short: "Manage project environment variables",
}

func init() {
	variablesCmd.AddCommand(variablesListCmd)
	variablesCmd.AddCommand(variablesSetCmd)
	variablesCmd.AddCommand(variablesDeleteCmd)
}

var variablesListCmd = &cobra.Command{
	Use:   "list <project>",
	Short: "List environment variables",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		var result map[string]any
		if err := apiClient.DoJSON("GET", fmt.Sprintf("/v1/projects/%s?expand=variables", args[0]), nil, &result); err != nil {
			return err
		}

		vars, _ := result["variables"].([]any)

		if outputJSON {
			printJSON(vars)
			return nil
		}

		if len(vars) == 0 {
			fmt.Println("No variables set.")
			return nil
		}

		w := newTabWriter()
		fmt.Fprintln(w, "ID\tNAME\tSECRET\tVALUE")
		for _, v := range vars {
			if vm, ok := v.(map[string]any); ok {
				value := vm["value"]
				if value == nil {
					value = "(hidden)"
				}
				fmt.Fprintf(w, "%v\t%v\t%v\t%v\n", vm["id"], vm["name"], vm["secret"], value)
			}
		}
		w.Flush()
		return nil
	},
}

var variablesSetCmd = &cobra.Command{
	Use:   "set <project> <name> <value>",
	Short: "Set an environment variable",
	Args:  cobra.ExactArgs(3),
	RunE: func(cmd *cobra.Command, args []string) error {
		secret, _ := cmd.Flags().GetBool("secret")
		body := map[string]any{
			"name":   args[1],
			"value":  args[2],
			"secret": secret,
		}
		data, _ := json.Marshal(body)

		if err := apiClient.DoJSON("POST", fmt.Sprintf("/v1/projects/%s/variables", args[0]), bytes.NewReader(data), nil); err != nil {
			return err
		}

		fmt.Printf("Variable %s set.\n", args[1])
		return nil
	},
}

func init() {
	variablesSetCmd.Flags().Bool("secret", false, "Mark variable as secret")
}

var variablesDeleteCmd = &cobra.Command{
	Use:   "delete <project> <variable-id>",
	Short: "Delete an environment variable",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := apiClient.DoJSON("DELETE", fmt.Sprintf("/v1/projects/%s/variables/%s", args[0], args[1]), nil, nil); err != nil {
			return err
		}

		fmt.Println("Variable deleted.")
		return nil
	},
}
