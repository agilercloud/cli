package cli

import (
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
		Use:   "list",
		Short: "List project domains",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			projectID, err := requireProjectID(a)
			if err != nil {
				return err
			}
			result, err := a.API.ListDomains(cmd.Context(), projectID)
			if err != nil {
				return err
			}
			renderDomainsList(a.Output, result)
			return nil
		},
	})

	addCmd := &cobra.Command{
		Use:   "add <domain>",
		Short: "Add a domain to a project",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			projectID, err := requireProjectID(a)
			if err != nil {
				return err
			}
			in := api.CreateDomainInput{Name: args[0]}
			if primary, _ := cmd.Flags().GetBool("primary"); primary {
				p := true
				in.Primary = &p
			}
			if _, err := a.API.CreateDomain(cmd.Context(), projectID, in); err != nil {
				return err
			}
			a.Output.Text("Domain %s added.", args[0])
			return nil
		},
	}
	addCmd.Flags().Bool("primary", false, "Mark this domain as primary")
	cmd.AddCommand(addCmd)

	cmd.AddCommand(&cobra.Command{
		Use:   "primary <domain-id>",
		Short: "Promote a domain to primary",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			projectID, err := requireProjectID(a)
			if err != nil {
				return err
			}
			p := true
			d, err := a.API.UpdateDomain(cmd.Context(), projectID, args[0], api.UpdateDomainInput{Primary: &p})
			if err != nil {
				return err
			}
			if a.Output.IsStructured() {
				a.Output.Structured(d)
			} else {
				a.Output.Text("Primary domain set to %s.", d.Name)
			}
			return nil
		},
	})

	cmd.AddCommand(&cobra.Command{
		Use:   "delete <domain-id>",
		Short: "Delete a domain from a project",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			projectID, err := requireProjectID(a)
			if err != nil {
				return err
			}
			if err := a.API.DeleteDomain(cmd.Context(), projectID, args[0]); err != nil {
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
		rows[i] = []string{d.Id.String(), d.Name, primary}
	}
	w.Table([]string{"ID", "NAME", "PRIMARY"}, rows)
}
