package cli

import (
	"strings"
	"time"

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
			result, err := a.API.GetSelfUser(cmd.Context())
			if err != nil {
				return err
			}
			return renderWhoami(a.Output, *result)
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
	w.Text("ID:      %s", u.Id)
	w.Text("Created: %s", u.CreatedAt.Format(time.RFC3339))
	if len(u.EffectiveScopes) == 0 {
		w.Text("Scopes:  (none)")
	} else {
		w.Text("Scopes:  %s", strings.Join(u.EffectiveScopes, ", "))
	}
	return nil
}
