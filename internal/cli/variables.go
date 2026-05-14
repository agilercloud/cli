package cli

import (
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
			result, err := a.API.ListVariables(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			renderVariablesList(a.Output, result)
			return nil
		},
	})

	setCmd := &cobra.Command{
		Use:   "set <project> <name> <value>",
		Short: "Create or update an environment variable",
		Args:  cobra.ExactArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			sensitive, _ := cmd.Flags().GetBool("sensitive")

			// Resolve to an existing variable if one with this name exists,
			// so `variables set` is an upsert across CLI sessions.
			existing, err := a.API.ListVariables(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			var variableId string
			for _, v := range existing {
				if v.Name == args[1] {
					variableId = v.Id.String()
					break
				}
			}

			if variableId == "" {
				if _, err := a.API.CreateVariable(cmd.Context(), args[0], api.CreateVariableInput{
					Name:      args[1],
					Value:     args[2],
					Sensitive: &sensitive,
				}); err != nil {
					return err
				}
			} else {
				name := args[1]
				value := args[2]
				if err := a.API.UpdateVariable(cmd.Context(), args[0], variableId, api.UpdateVariableInput{
					Name:      &name,
					Value:     &value,
					Sensitive: &sensitive,
				}); err != nil {
					return err
				}
			}
			a.Output.Text("Variable %s set.", args[1])
			return nil
		},
	}
	setCmd.Flags().Bool("sensitive", false, "Mark variable as sensitive")
	cmd.AddCommand(setCmd)

	cmd.AddCommand(&cobra.Command{
		Use:   "delete <project> <variable-id>",
		Short: "Delete an environment variable",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := a.API.DeleteVariable(cmd.Context(), args[0], args[1]); err != nil {
				return err
			}
			a.Output.Text("Variable deleted.")
			return nil
		},
	})

	return cmd
}

func renderVariablesList(w *output.Writer, vars []api.Variable) {
	if w.IsStructured() {
		w.Structured(vars)
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
		rows[i] = []string{v.Id.String(), v.Name, fmt.Sprintf("%t", v.Sensitive), value}
	}
	w.Table([]string{"ID", "NAME", "SENSITIVE", "VALUE"}, rows)
}
