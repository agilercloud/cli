package cli

import (
	"fmt"
	"strings"

	"github.com/agilercloud/cli/internal/api"
	"github.com/agilercloud/cli/internal/app"
	"github.com/agilercloud/cli/internal/output"
	"github.com/spf13/cobra"
)

func newVariablesCmd(a *app.App) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "variables",
		Aliases: []string{"variable"},
		Short:   "Manage project environment variables",
	}

	cmd.AddCommand(&cobra.Command{
		Use:   "list",
		Short: "List environment variables",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			projectID, err := requireProjectID(a)
			if err != nil {
				return err
			}
			result, err := a.API.ListVariables(cmd.Context(), projectID)
			if err != nil {
				return err
			}
			renderVariablesList(a.Output, result)
			return nil
		},
	})

	setCmd := &cobra.Command{
		Use:   "set <name> <value>",
		Short: "Create or update an environment variable",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			projectID, err := requireProjectID(a)
			if err != nil {
				return err
			}
			sensitive, _ := cmd.Flags().GetBool("sensitive")
			// Only forward the sensitive field when the flag was explicitly
			// provided; otherwise leave it nil so the server preserves the
			// existing value on update (and applies its default on create).
			var sensitivePtr *bool
			if cmd.Flags().Changed("sensitive") {
				sensitivePtr = &sensitive
			}

			name := strings.ToUpper(strings.TrimSpace(args[0]))
			value := args[1]

			existing, err := a.API.ListVariables(cmd.Context(), projectID)
			if err != nil {
				return err
			}
			var variableId string
			for _, v := range existing {
				if v.Name == name {
					variableId = v.Id.String()
					break
				}
			}

			var result *api.Variable
			if variableId == "" {
				v, err := a.API.CreateVariable(cmd.Context(), projectID, api.CreateVariableInput{
					Name:      name,
					Value:     value,
					Sensitive: sensitivePtr,
				})
				if err != nil {
					return err
				}
				result = v
			} else {
				v, err := a.API.UpdateVariable(cmd.Context(), projectID, variableId, api.UpdateVariableInput{
					Name:      &name,
					Value:     &value,
					Sensitive: sensitivePtr,
				})
				if err != nil {
					return err
				}
				result = v
			}
			if a.Output.IsStructured() {
				a.Output.Structured(result)
			} else if a.Output.IsQuiet() {
				a.Output.Text("%s", result.Id)
			} else {
				a.Output.Text("Variable %s set (sensitive=%s): %s", result.Name, output.YesNo(result.Sensitive), result.Id)
			}
			return nil
		},
	}
	setCmd.Flags().Bool("sensitive", false, "Mark variable as sensitive")
	cmd.AddCommand(setCmd)

	deleteCmd := &cobra.Command{
		Use:   "delete <variable-id>",
		Short: "Delete an environment variable",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			projectID, err := requireProjectID(a)
			if err != nil {
				return err
			}
			if err := confirmOrSkip(a, cmd, fmt.Sprintf("Delete variable %s? (y/N) ", args[0])); err != nil {
				return err
			}
			if err := a.API.DeleteVariable(cmd.Context(), projectID, args[0]); err != nil {
				return err
			}
			a.Output.Text("Variable deleted.")
			return nil
		},
	}
	addYesFlag(deleteCmd)
	cmd.AddCommand(deleteCmd)

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
		rows[i] = []string{v.Id.String(), v.Name, output.YesNo(v.Sensitive), value}
	}
	w.Table([]string{"ID", "NAME", "SENSITIVE", "VALUE"}, rows)
}
