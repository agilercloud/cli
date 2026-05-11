package cli

import (
	"bytes"
	"encoding/json"
	"fmt"

	"github.com/agilercloud/cli/internal/api"
	"github.com/agilercloud/cli/internal/app"
	"github.com/agilercloud/cli/internal/output"
	"github.com/spf13/cobra"
)

func newDomainsCmd(a *app.App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "domains",
		Short: "Manage project domains",
	}

	cmd.AddCommand(&cobra.Command{
		Use:   "list <project>",
		Short: "List project domains",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			var result []api.Domain
			if err := a.API.DoJSON(cmd.Context(), "GET", fmt.Sprintf("/v1/projects/%s/domains", args[0]), nil, &result); err != nil {
				return err
			}
			renderDomainsList(a.Output, result)
			return nil
		},
	})

	addCmd := &cobra.Command{
		Use:   "add <project> <domain>",
		Short: "Add a domain to a project",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			primary, _ := cmd.Flags().GetBool("primary")
			body := map[string]any{"name": args[1]}
			if primary {
				body["primary"] = true
			}
			data, _ := json.Marshal(body)

			if err := a.API.DoJSONIdempotent(cmd.Context(), "POST", fmt.Sprintf("/v1/projects/%s/domains", args[0]), bytes.NewReader(data), nil); err != nil {
				return err
			}
			a.Output.Text("Domain %s added.", args[1])
			return nil
		},
	}
	addCmd.Flags().Bool("primary", false, "Mark this domain as primary")
	cmd.AddCommand(addCmd)

	cmd.AddCommand(&cobra.Command{
		Use:   "primary <project> <domain-id>",
		Short: "Promote a domain to primary",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			body, _ := json.Marshal(map[string]any{"primary": true})
			if err := a.API.DoJSON(cmd.Context(), "PATCH", fmt.Sprintf("/v1/projects/%s/domains/%s", args[0], args[1]), bytes.NewReader(body), nil); err != nil {
				return err
			}
			a.Output.Text("Primary domain set.")
			return nil
		},
	})

	cmd.AddCommand(&cobra.Command{
		Use:   "delete <project> <domain-id>",
		Short: "Delete a domain from a project",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := a.API.DoJSON(cmd.Context(), "DELETE", fmt.Sprintf("/v1/projects/%s/domains/%s", args[0], args[1]), nil, nil); err != nil {
				return err
			}
			a.Output.Text("Domain deleted.")
			return nil
		},
	})

	return cmd
}

func renderDomainsList(w *output.Writer, domains []api.Domain) {
	if w.IsStructured() {
		w.Structured(domains)
		return
	}
	if len(domains) == 0 {
		w.Text("No domains configured.")
		return
	}
	rows := make([][]string, len(domains))
	for i, d := range domains {
		primary := ""
		if d.Primary {
			primary = "yes"
		}
		rows[i] = []string{d.ID, d.Name, primary}
	}
	w.Table([]string{"ID", "NAME", "PRIMARY"}, rows)
}
