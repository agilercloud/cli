package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Check API status",
	RunE: func(cmd *cobra.Command, args []string) error {
		var result map[string]any
		if err := apiClient.DoJSON("GET", "/status", nil, &result); err != nil {
			return fmt.Errorf("status check failed: %w", err)
		}

		if outputJSON {
			printJSON(result)
			return nil
		}

		status, _ := result["status"].(string)
		fmt.Printf("status: %s\n", status)
		return nil
	},
}
