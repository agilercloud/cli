package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

var regionsCmd = &cobra.Command{
	Use:   "regions",
	Short: "List available regions",
}

func init() {
	regionsCmd.AddCommand(regionsListCmd)
	regionsCmd.AddCommand(regionsGetCmd)
}

var regionsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all regions",
	RunE: func(cmd *cobra.Command, args []string) error {
		var result []map[string]any
		if err := apiClient.DoJSON("GET", "/v1/regions", nil, &result); err != nil {
			return err
		}

		if outputJSON {
			printJSON(result)
			return nil
		}

		if len(result) == 0 {
			fmt.Println("No regions available.")
			return nil
		}

		if outputQuiet {
			for _, r := range result {
				fmt.Println(r["id"])
			}
			return nil
		}

		w := newTabWriter()
		fmt.Fprintln(w, "ID\tDESCRIPTION")
		for _, r := range result {
			fmt.Fprintf(w, "%v\t%v\n", r["id"], r["description"])
		}
		w.Flush()
		return nil
	},
}

var regionsGetCmd = &cobra.Command{
	Use:   "get <region-id>",
	Short: "Get region details",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		var result map[string]any
		if err := apiClient.DoJSON("GET", fmt.Sprintf("/v1/regions/%s", args[0]), nil, &result); err != nil {
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
		return nil
	},
}
