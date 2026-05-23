package cli

import (
	"github.com/agilercloud/cli/internal/app"
	"github.com/spf13/cobra"
)

// completeProjectIDs returns a Cobra completion function that lists
// project IDs visible to the configured API key, scoped to the
// configured workspace when one is set.
func completeProjectIDs(a *app.App) cobra.CompletionFunc {
	return func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		if err := ensureAPI(a); err != nil {
			return nil, cobra.ShellCompDirectiveError
		}
		workspaceID, err := normalizeWorkspaceID(configuredWorkspaceID(a))
		if err != nil {
			workspaceID = ""
		}
		projects, err := a.API.ListProjects(cmd.Context(), workspaceID)
		if err != nil {
			return nil, cobra.ShellCompDirectiveError
		}
		out := make([]string, 0, len(projects))
		for _, p := range projects {
			out = append(out, p.Id.String()+"\t"+p.Name)
		}
		return out, cobra.ShellCompDirectiveNoFileComp
	}
}

// completeWorkspaceIDs returns a Cobra completion function that lists
// workspace IDs visible to the configured API key.
func completeWorkspaceIDs(a *app.App) cobra.CompletionFunc {
	return func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		if err := ensureAPI(a); err != nil {
			return nil, cobra.ShellCompDirectiveError
		}
		workspaces, err := a.API.ListWorkspaces(cmd.Context())
		if err != nil {
			return nil, cobra.ShellCompDirectiveError
		}
		out := make([]string, 0, len(workspaces))
		for _, w := range workspaces {
			out = append(out, w.Id.String()+"\t"+w.Name)
		}
		return out, cobra.ShellCompDirectiveNoFileComp
	}
}

// completeRegionIDs returns a Cobra completion function that lists
// region IDs from the platform catalog.
func completeRegionIDs(a *app.App) cobra.CompletionFunc {
	return func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		if err := ensureAPI(a); err != nil {
			return nil, cobra.ShellCompDirectiveError
		}
		regions, err := a.API.ListRegions(cmd.Context())
		if err != nil {
			return nil, cobra.ShellCompDirectiveError
		}
		out := make([]string, 0, len(regions))
		for _, r := range regions {
			out = append(out, r.Id+"\t"+r.Description)
		}
		return out, cobra.ShellCompDirectiveNoFileComp
	}
}

// completeRuntimeIDs returns a Cobra completion function that lists
// runtime IDs from the platform catalog.
func completeRuntimeIDs(a *app.App) cobra.CompletionFunc {
	return func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		if err := ensureAPI(a); err != nil {
			return nil, cobra.ShellCompDirectiveError
		}
		runtimes, err := a.API.ListRuntimes(cmd.Context())
		if err != nil {
			return nil, cobra.ShellCompDirectiveError
		}
		out := make([]string, 0, len(runtimes))
		for _, r := range runtimes {
			out = append(out, r.Id+"\t"+r.Description)
		}
		return out, cobra.ShellCompDirectiveNoFileComp
	}
}
