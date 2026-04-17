package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

var runtimesCmd = &cobra.Command{
	Use:   "runtimes",
	Short: "List available runtimes",
}

func init() {
	runtimesCmd.AddCommand(runtimesListCmd)
	runtimesCmd.AddCommand(runtimesGetCmd)
}

var runtimesListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all runtimes",
	RunE: func(cmd *cobra.Command, args []string) error {
		var result []map[string]any
		if err := apiClient.DoJSON("GET", "/v1/runtimes", nil, &result); err != nil {
			return err
		}

		if outputJSON {
			printJSON(result)
			return nil
		}

		if len(result) == 0 {
			fmt.Println("No runtimes available.")
			return nil
		}

		if outputQuiet {
			for _, r := range result {
				fmt.Println(r["id"])
			}
			return nil
		}

		w := newTabWriter()
		fmt.Fprintln(w, "ID\tDESCRIPTION\tDEPRECATED")
		for _, r := range result {
			deprecated := ""
			if v := r["deprecated_at"]; v != nil {
				deprecated = fmt.Sprintf("%v", v)
			}
			fmt.Fprintf(w, "%v\t%v\t%v\n", r["id"], r["description"], deprecated)
		}
		w.Flush()
		return nil
	},
}

var runtimesGetCmd = &cobra.Command{
	Use:   "get <runtime-id>",
	Short: "Get runtime details",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		var result map[string]any
		if err := apiClient.DoJSON("GET", fmt.Sprintf("/v1/runtimes/%s", args[0]), nil, &result); err != nil {
			return err
		}

		if outputJSON {
			printJSON(result)
			return nil
		}

		fmt.Printf("ID:          %v\n", result["id"])
		fmt.Printf("Description: %v\n", result["description"])
		fmt.Printf("Created:     %v\n", result["created_at"])
		fmt.Printf("Updated:     %v\n", result["updated_at"])
		if v := result["deprecated_at"]; v != nil {
			fmt.Printf("Deprecated:  %v\n", v)
		}
		return nil
	},
}
