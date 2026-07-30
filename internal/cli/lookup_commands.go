package cli

import (
	"context"

	"github.com/spf13/cobra"
)

// lookupCommandOptions contains explicit Cobra metadata and already-written
// resource callbacks for the narrow list/get shape shared by reference data.
type lookupCommandOptions[T any] struct {
	Use        string
	Aliases    []string
	Short      string
	ListShort  string
	GetUse     string
	GetShort   string
	Complete   func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective)
	List       func(context.Context) ([]T, error)
	Get        func(context.Context, string) (*T, error)
	RenderList func([]T)
	RenderGet  func(T) error
}

func newLookupCommand[T any](opts lookupCommandOptions[T]) *cobra.Command {
	cmd := &cobra.Command{
		Use:     opts.Use,
		Aliases: opts.Aliases,
		Short:   opts.Short,
	}

	cmd.AddCommand(&cobra.Command{
		Use:   "list",
		Short: opts.ListShort,
		RunE: func(cmd *cobra.Command, args []string) error {
			result, err := opts.List(cmd.Context())
			if err != nil {
				return err
			}
			opts.RenderList(result)
			return nil
		},
	})

	cmd.AddCommand(&cobra.Command{
		Use:               opts.GetUse,
		Short:             opts.GetShort,
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: opts.Complete,
		RunE: func(cmd *cobra.Command, args []string) error {
			result, err := opts.Get(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			return opts.RenderGet(*result)
		},
	})

	return cmd
}
