package cli

import (
	"time"

	"github.com/agilercloud/cli/internal/api"
	"github.com/agilercloud/cli/internal/app"
	"github.com/agilercloud/cli/internal/output"
	"github.com/spf13/cobra"
)

func newWorkspacesCmd(a *app.App) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "workspaces",
		Aliases: []string{"workspace"},
		Short:   "Manage workspaces",
	}

	cmd.AddCommand(newWorkspacesListCmd(a))
	cmd.AddCommand(newWorkspacesGetCmd(a))
	cmd.AddCommand(newWorkspacesCreateCmd(a))
	cmd.AddCommand(newWorkspacesMembersCmd(a))

	return cmd
}

func newWorkspacesListCmd(a *app.App) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List workspaces",
		RunE: func(cmd *cobra.Command, args []string) error {
			result, err := a.API.ListWorkspaces(cmd.Context())
			if err != nil {
				return err
			}
			renderWorkspacesList(a.Output, result)
			return nil
		},
	}
}

func newWorkspacesGetCmd(a *app.App) *cobra.Command {
	return &cobra.Command{
		Use:               "get <workspace>",
		Short:             "Get workspace details",
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: completeWorkspaceIDs(a),
		RunE: func(cmd *cobra.Command, args []string) error {
			workspaceID, err := normalizeWorkspaceID(args[0])
			if err != nil {
				return err
			}
			result, err := a.API.GetWorkspace(cmd.Context(), workspaceID)
			if err != nil {
				return err
			}
			return renderWorkspaceDetail(a.Output, *result)
		},
	}
}

func newWorkspacesCreateCmd(a *app.App) *cobra.Command {
	var name string

	cmd := &cobra.Command{
		Use:     "create",
		Short:   "Create a workspace",
		Long:    "Create a new workspace. The caller becomes its first member with the Owner role.",
		Example: `  agiler workspaces create --name acme`,
		RunE: func(cmd *cobra.Command, args []string) error {
			result, err := a.API.CreateWorkspace(cmd.Context(), api.CreateWorkspaceInput{Name: name})
			if err != nil {
				return err
			}
			if a.Output.IsStructured() {
				a.Output.Structured(result)
			} else if a.Output.IsQuiet() {
				a.Output.Text("%s", result.Id)
			} else {
				a.Output.Text("Workspace created: %s", result.Id)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&name, "name", "", "Workspace name (required)")
	_ = cmd.MarkFlagRequired("name")
	return cmd
}

func newWorkspacesMembersCmd(a *app.App) *cobra.Command {
	return &cobra.Command{
		Use:               "members <workspace>",
		Short:             "List workspace members and pending invites",
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: completeWorkspaceIDs(a),
		RunE: func(cmd *cobra.Command, args []string) error {
			workspaceID, err := normalizeWorkspaceID(args[0])
			if err != nil {
				return err
			}
			result, err := a.API.ListWorkspaceMembers(cmd.Context(), workspaceID)
			if err != nil {
				return err
			}
			renderWorkspaceMembersList(a.Output, result)
			return nil
		},
	}
}

func renderWorkspacesList(w *output.Writer, ws []api.Workspace) {
	if w.IsStructured() {
		w.Structured(ws)
		return
	}
	if len(ws) == 0 {
		w.Text("No workspaces found.")
		return
	}
	rows := make([][]string, len(ws))
	for i, s := range ws {
		rows[i] = []string{
			s.Id.String(),
			s.Name,
			s.Role,
			output.YesNo(s.IsBillingUser),
			output.YesNo(s.RequireMfa),
			output.YesNo(s.MfaRequiredForCaller),
		}
	}
	w.Table([]string{"ID", "NAME", "ROLE", "BILLING", "REQUIRE MFA", "MFA REQUIRED"}, rows)
}

func renderWorkspaceDetail(w *output.Writer, s api.Workspace) error {
	if w.IsTabular() {
		return tabularUnsupportedErr(w)
	}
	if w.IsStructured() {
		w.Structured(s)
		return nil
	}
	if w.IsQuiet() {
		w.Text("%s", s.Id)
		return nil
	}
	w.Text("%s %s", w.OutColor.Dim("ID:             "), s.Id)
	w.Text("%s %s", w.OutColor.Dim("Name:           "), s.Name)
	w.Text("%s %s", w.OutColor.Dim("Role:           "), s.Role)
	w.Text("%s %s", w.OutColor.Dim("Billing User:   "), output.YesNo(s.IsBillingUser))
	w.Text("%s %s", w.OutColor.Dim("Billing User ID:"), s.BillingUserId)
	w.Text("%s %s", w.OutColor.Dim("Require MFA:    "), output.YesNo(s.RequireMfa))
	w.Text("%s %s", w.OutColor.Dim("MFA Required:   "), output.YesNo(s.MfaRequiredForCaller))
	w.Text("%s %s", w.OutColor.Dim("Created:        "), s.CreatedAt.Format(time.RFC3339))
	w.Text("%s %s", w.OutColor.Dim("Updated:        "), s.UpdatedAt.Format(time.RFC3339))
	return nil
}

func renderWorkspaceMembersList(w *output.Writer, members []api.WorkspaceMember) {
	if w.IsStructured() {
		w.Structured(members)
		return
	}
	if len(members) == 0 {
		w.Text("No workspace members found.")
		return
	}
	rows := make([][]string, len(members))
	for i, m := range members {
		rows[i] = []string{
			workspaceMemberID(m),
			m.Email,
			optionalString(m.Name),
			m.Role,
			m.Status,
			output.YesNo(m.IsBillingUser),
			optionalBool(m.MfaEnabled),
		}
	}
	w.Table([]string{"ID", "EMAIL", "NAME", "ROLE", "STATUS", "BILLING", "MFA"}, rows)
}

func workspaceMemberID(m api.WorkspaceMember) string {
	if m.UserId != nil {
		return m.UserId.String()
	}
	if m.InviteId != nil {
		return m.InviteId.String()
	}
	return ""
}

func optionalString(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func optionalBool(value *bool) string {
	if value == nil {
		return ""
	}
	return output.YesNo(*value)
}
