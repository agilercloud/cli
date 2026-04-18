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

func newVariablesCmd(a *app.App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "variables",
		Short: "Manage project environment variables",
	}

	cmd.AddCommand(&cobra.Command{
		Use:   "list <project>",
		Short: "List environment variables",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			var result api.Project
			if err := a.API.DoJSON(cmd.Context(), "GET", fmt.Sprintf("/v1/projects/%s?expand=variables", args[0]), nil, &result); err != nil {
				return err
			}
			renderVariablesList(a.Output, result.Variables)
			return nil
		},
	})

	setCmd := &cobra.Command{
		Use:   "set <project> <name> <value>",
		Short: "Set an environment variable",
		Args:  cobra.ExactArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			secret, _ := cmd.Flags().GetBool("secret")
			body := map[string]any{
				"name":   args[1],
				"value":  args[2],
				"secret": secret,
			}
			data, _ := json.Marshal(body)

			if err := a.API.DoJSON(cmd.Context(), "POST", fmt.Sprintf("/v1/projects/%s/variables", args[0]), bytes.NewReader(data), nil); err != nil {
				return err
			}
			a.Output.Text("Variable %s set.", args[1])
			return nil
		},
	}
	setCmd.Flags().Bool("secret", false, "Mark variable as secret")
	cmd.AddCommand(setCmd)

	cmd.AddCommand(&cobra.Command{
		Use:   "delete <project> <variable-id>",
		Short: "Delete an environment variable",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := a.API.DoJSON(cmd.Context(), "DELETE", fmt.Sprintf("/v1/projects/%s/variables/%s", args[0], args[1]), nil, nil); err != nil {
				return err
			}
			a.Output.Text("Variable deleted.")
			return nil
		},
	})

	return cmd
}

func renderVariablesList(w *output.Writer, vars []api.Variable) {
	if w.IsJSON() {
		w.JSON(vars)
		return
	}
	if len(vars) == 0 {
		w.Text("No variables set.")
		return
	}
	rows := make([][]string, len(vars))
	for i, v := range vars {
		value := "(hidden)"
		if v.Value != nil {
			value = *v.Value
		}
		rows[i] = []string{v.ID, v.Name, fmt.Sprintf("%t", v.Secret), value}
	}
	w.Table([]string{"ID", "NAME", "SECRET", "VALUE"}, rows)
}
