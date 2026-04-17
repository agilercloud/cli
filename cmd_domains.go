package main

import (
	"bytes"
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
)

var domainsCmd = &cobra.Command{
	Use:   "domains",
	Short: "Manage project domains",
}

func init() {
	domainsCmd.AddCommand(domainsListCmd)
	domainsCmd.AddCommand(domainsAddCmd)
	domainsCmd.AddCommand(domainsDeleteCmd)
}

var domainsListCmd = &cobra.Command{
	Use:   "list <project>",
	Short: "List project domains",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		var result map[string]any
		if err := apiClient.DoJSON("GET", fmt.Sprintf("/v1/projects/%s", args[0]), nil, &result); err != nil {
			return err
		}

		domains, _ := result["domains"].([]any)

		if outputJSON {
			printJSON(domains)
			return nil
		}

		if len(domains) == 0 {
			fmt.Println("No domains configured.")
			return nil
		}

		w := newTabWriter()
		fmt.Fprintln(w, "ID\tNAME")
		for _, d := range domains {
			if dm, ok := d.(map[string]any); ok {
				fmt.Fprintf(w, "%v\t%v\n", dm["id"], dm["name"])
			}
		}
		w.Flush()
		return nil
	},
}

var domainsAddCmd = &cobra.Command{
	Use:   "add <project> <domain>",
	Short: "Add a domain to a project",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		body := map[string]any{
			"domains": []string{args[1]},
		}
		data, _ := json.Marshal(body)

		if err := apiClient.DoJSON("POST", fmt.Sprintf("/v1/projects/%s/domains", args[0]), bytes.NewReader(data), nil); err != nil {
			return err
		}

		fmt.Printf("Domain %s added.\n", args[1])
		return nil
	},
}

var domainsDeleteCmd = &cobra.Command{
	Use:   "delete <project> <domain-id>",
	Short: "Delete a domain from a project",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := apiClient.DoJSON("DELETE", fmt.Sprintf("/v1/projects/%s/domains/%s", args[0], args[1]), nil, nil); err != nil {
			return err
		}

		fmt.Println("Domain deleted.")
		return nil
	},
}
