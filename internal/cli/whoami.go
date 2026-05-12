package cli

import (
	"strings"

	"github.com/agilercloud/cli/internal/api"
	"github.com/agilercloud/cli/internal/app"
	"github.com/agilercloud/cli/internal/output"
	"github.com/spf13/cobra"
)

func newWhoamiCmd(a *app.App) *cobra.Command {
	return &cobra.Command{
		Use:   "whoami",
		Short: "Show the authenticated user and effective scopes",
		Long: "Calls GET /v1/users/me and prints the user the current credentials authenticate as, " +
			"plus the effective permission scopes those credentials grant. " +
			"Useful for diagnosing 403s — if a command fails with 'forbidden', this lists which scopes the key holds.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			var result api.SelfUser
			if err := a.API.DoJSON(cmd.Context(), "GET", "/v1/users/me", nil, &result); err != nil {
				return err
			}
			return renderWhoami(a.Output, result)
		},
	}
}

func renderWhoami(w *output.Writer, u api.SelfUser) error {
	if w.IsTabular() {
		return tabularUnsupportedErr(w)
	}
	if w.IsStructured() {
		w.Structured(u)
		return nil
	}
	w.Text("User:    %s", u.Email)
	w.Text("Name:    %s", u.Name)
	w.Text("ID:      %s", u.ID)
	w.Text("Created: %s", u.CreatedAt)
	if u.SiteAdmin {
		w.Text("Site admin: yes")
	}
	if len(u.EffectiveScopes) == 0 {
		w.Text("Scopes:  (none)")
	} else {
		w.Text("Scopes:  %s", strings.Join(u.EffectiveScopes, ", "))
	}
	return nil
}
