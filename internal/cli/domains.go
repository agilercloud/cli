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
			var result api.Project
			if err := a.API.DoJSON(cmd.Context(), "GET", fmt.Sprintf("/v1/projects/%s", args[0]), nil, &result); err != nil {
				return err
			}
			renderDomainsList(a.Output, result.Domains)
			return nil
		},
	})

	cmd.AddCommand(&cobra.Command{
		Use:   "add <project> <domain>",
		Short: "Add a domain to a project",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			body := map[string]any{"domains": []string{args[1]}}
			data, _ := json.Marshal(body)

			if err := a.API.DoJSON(cmd.Context(), "POST", fmt.Sprintf("/v1/projects/%s/domains", args[0]), bytes.NewReader(data), nil); err != nil {
				return err
			}
			a.Output.Text("Domain %s added.", args[1])
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
	if w.IsJSON() {
		w.JSON(domains)
		return
	}
	if len(domains) == 0 {
		w.Text("No domains configured.")
		return
	}
	rows := make([][]string, len(domains))
	for i, d := range domains {
		rows[i] = []string{d.ID, d.Name}
	}
	w.Table([]string{"ID", "NAME"}, rows)
}
