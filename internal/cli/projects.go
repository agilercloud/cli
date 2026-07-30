package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"time"

	"github.com/agilercloud/cli/internal/api"
	"github.com/agilercloud/cli/internal/app"
	"github.com/agilercloud/cli/internal/output"
	"github.com/google/uuid"
	"github.com/spf13/cobra"
)

func newProjectsCmd(a *app.App) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "projects",
		Aliases: []string{"project"},
		Short:   "Manage projects",
	}

	cmd.AddCommand(newProjectsListCmd(a))
	cmd.AddCommand(newProjectsGetCmd(a))
	cmd.AddCommand(newProjectsCreateCmd(a))
	cmd.AddCommand(newProjectsUpdateCmd(a))
	cmd.AddCommand(newProjectsDeleteCmd(a))

	return cmd
}

func newProjectsListCmd(a *app.App) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List all projects",
		RunE: func(cmd *cobra.Command, args []string) error {
			workspaceID, err := normalizeWorkspaceID(configuredWorkspaceID(a))
			if err != nil {
				return err
			}
			result, err := a.API.ListProjects(cmd.Context(), workspaceID)
			if err != nil {
				return err
			}
			renderProjectsList(a.Output, result)
			return nil
		},
	}
}

func newProjectsGetCmd(a *app.App) *cobra.Command {
	return &cobra.Command{
		Use:               "get <project>",
		Short:             "Get project details",
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: completeProjectIDs(a),
		RunE: func(cmd *cobra.Command, args []string) error {
			result, err := a.API.GetProject(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			return renderProjectDetail(a.Output, *result)
		},
	}
}

func newProjectsCreateCmd(a *app.App) *cobra.Command {
	var name, region, runtime string

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a new project",
		Long:  "Create a new project. --name, --region, and --runtime are required. Use agiler regions list to discover available regions, and agiler runtimes list to discover available runtimes.",
		Example: `  agiler regions list
  agiler runtimes list
  agiler projects create --name api --region us-east-1 --runtime node-20`,
		RunE: func(cmd *cobra.Command, args []string) error {
			in := api.CreateProjectInput{
				Name:    name,
				Region:  region,
				Runtime: runtime,
			}
			if workspaceID, err := parseWorkspaceID(configuredWorkspaceID(a)); err != nil {
				return err
			} else if workspaceID != uuid.Nil {
				in.WorkspaceId = &workspaceID
			}
			result, err := a.API.CreateProject(cmd.Context(), in)
			if err != nil {
				return err
			}
			return renderProjectDetail(a.Output, *result)
		},
	}
	cmd.Flags().StringVar(&name, "name", "", "Project name (required)")
	cmd.Flags().StringVar(&region, "region", "", "Region ID (required)")
	cmd.Flags().StringVar(&runtime, "runtime", "", "Runtime ID (required)")
	_ = cmd.MarkFlagRequired("name")
	_ = cmd.MarkFlagRequired("region")
	_ = cmd.MarkFlagRequired("runtime")
	_ = cmd.RegisterFlagCompletionFunc("region", completeRegionIDs(a))
	_ = cmd.RegisterFlagCompletionFunc("runtime", completeRuntimeIDs(a))
	return cmd
}

func newProjectsUpdateCmd(a *app.App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "update <project>",
		Short: "Update a project",
		Long:  "Update fields on an existing project. Pass only the flags whose values should change; omitted flags are left untouched. Use --target-workspace to move the project to a different workspace.",
		Example: `  agiler projects update <pid> --name api-prod
  agiler projects update <pid> --runtime node-22
  agiler projects update <pid> --target-workspace <wid>
  agiler projects update <pid> --active=false`,
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: completeProjectIDs(a),
		RunE: func(cmd *cobra.Command, args []string) error {
			var in api.UpdateProjectInput
			touched := false

			if cmd.Flags().Changed("name") {
				v, _ := cmd.Flags().GetString("name")
				in.Name = &v
				touched = true
			}
			if cmd.Flags().Changed("active") {
				v, _ := cmd.Flags().GetBool("active")
				in.Active = &v
				touched = true
			}
			if cmd.Flags().Changed("runtime") {
				v, _ := cmd.Flags().GetString("runtime")
				in.Runtime = &v
				touched = true
			}
			if cmd.Flags().Changed("target-workspace") {
				raw, _ := cmd.Flags().GetString("target-workspace")
				v, err := normalizeWorkspaceID(raw)
				if err != nil {
					return err
				}
				if v == "" {
					return fmt.Errorf("--target-workspace must be a valid UUID")
				}
				in.WorkspaceId = &v
				touched = true
			}

			if !touched {
				return fmt.Errorf("no flags provided; use --name, --active, --runtime, or --target-workspace")
			}

			result, err := a.API.UpdateProject(cmd.Context(), args[0], in)
			if err != nil {
				return err
			}
			return renderProjectDetail(a.Output, *result)
		},
	}
	cmd.Flags().String("name", "", "Project name")
	cmd.Flags().Bool("active", false, "Active state")
	cmd.Flags().String("runtime", "", "Runtime ID")
	cmd.Flags().String("target-workspace", "", "Move the project into this workspace (UUID)")
	_ = cmd.RegisterFlagCompletionFunc("runtime", completeRuntimeIDs(a))
	_ = cmd.RegisterFlagCompletionFunc("target-workspace", completeWorkspaceIDs(a))
	return cmd
}

func newProjectsDeleteCmd(a *app.App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "delete <project>",
		Short: "Delete a project",
		Long:  "Delete a project and all of its resources. The command requires confirmation; pass --yes to skip.",
		Example: `  agiler projects delete <pid>
  agiler projects delete <pid> --yes`,
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: completeProjectIDs(a),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := confirmOrSkip(a, cmd, fmt.Sprintf("Delete project %s? (y/N) ", args[0])); err != nil {
				return err
			}
			if err := a.API.DeleteProject(cmd.Context(), args[0]); err != nil {
				return err
			}
			a.Output.Text("Project deleted.")
			return nil
		},
	}
	addYesFlag(cmd)
	return cmd
}

func newUsageCmd(a *app.App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "usage",
		Short: "Get project usage statistics",
		Long:  "Fetch project usage bucketed by --granularity (hour|day|week|month). --since/--until accept RFC3339 timestamps or durations (e.g. 1h, 24h); --limit caps page size.",
		Example: `  agiler usage
  agiler usage --granularity hour --since 24h
  agiler usage --granularity month --since 2026-01-01T00:00:00Z`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			projectID, err := requireProjectID(a)
			if err != nil {
				return err
			}
			q := api.UsageQuery{}
			if v, _ := cmd.Flags().GetInt("limit"); v > 0 {
				q.Limit = v
			}
			if v, _ := cmd.Flags().GetString("since"); v != "" {
				t, err := parseTimeFlag(v, a.Clock.Now())
				if err != nil {
					return fmt.Errorf("invalid --since: %w", err)
				}
				q.Since = t
			}
			if v, _ := cmd.Flags().GetString("until"); v != "" {
				t, err := parseTimeFlag(v, a.Clock.Now())
				if err != nil {
					return fmt.Errorf("invalid --until: %w", err)
				}
				q.Until = t
			}
			if v, _ := cmd.Flags().GetString("granularity"); v != "" {
				q.Granularity = api.UsageGranularity(v)
			}
			result, err := a.API.GetProjectUsage(cmd.Context(), projectID, q)
			if err != nil {
				return err
			}
			renderUsageList(a.Output, result)
			return nil
		},
	}
	cmd.Flags().Int("limit", 0, "Page size (0 = server default, max 365)")
	cmd.Flags().String("since", "", "Start of the window (RFC3339 or duration like 1h, 24h)")
	cmd.Flags().String("until", "", "End of the window (RFC3339 or duration like 1h, 24h)")
	cmd.Flags().String("granularity", "day", "Bucket size: hour|day|week|month (default day)")
	return cmd
}

func newLogsCmd(a *app.App) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "logs",
		Aliases: []string{"log"},
		Short:   "Get project logs",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			projectID, err := requireProjectID(a)
			if err != nil {
				return err
			}
			q := api.LogsQuery{}
			if v, _ := cmd.Flags().GetInt("limit"); v > 0 {
				q.Limit = v
			}
			if v, _ := cmd.Flags().GetString("since"); v != "" {
				t, err := parseTimeFlag(v, a.Clock.Now())
				if err != nil {
					return fmt.Errorf("invalid --since: %w", err)
				}
				q.Since = t
			}
			if v, _ := cmd.Flags().GetString("until"); v != "" {
				t, err := parseTimeFlag(v, a.Clock.Now())
				if err != nil {
					return fmt.Errorf("invalid --until: %w", err)
				}
				q.Until = t
			}
			return runLogsQuery(cmd.Context(), a, projectID, q)
		},
	}
	cmd.Flags().Int("limit", 0, "Maximum entries returned (0 = server default)")
	cmd.Flags().String("since", "", "Start of the window (RFC3339 or duration like 1h, 24h)")
	cmd.Flags().String("until", "", "End of the window (RFC3339 or duration like 1h, 24h)")
	cmd.AddCommand(newLogsTailCmd(a))
	cmd.AddCommand(newLogsSearchCmd(a))
	return cmd
}

func newLogsTailCmd(a *app.App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "tail",
		Short: "Stream project logs in real-time",
		Long:  "Stream project logs in real time, polling on --interval. New entries print as they arrive. Press Ctrl-C to stop. Use --since to backfill before tailing forward.",
		Example: `  agiler logs tail
  agiler logs tail --since 5m
  agiler logs tail --interval 5s --format json`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			projectID, err := requireProjectID(a)
			if err != nil {
				return err
			}
			interval, _ := cmd.Flags().GetString("interval")
			pollInterval, err := time.ParseDuration(interval)
			if err != nil {
				return fmt.Errorf("invalid interval: %w", err)
			}

			sinceFlag, _ := cmd.Flags().GetString("since")
			since := a.Clock.Now().UTC()
			if sinceFlag != "" {
				t, err := parseTimeFlag(sinceFlag, a.Clock.Now())
				if err != nil {
					return fmt.Errorf("invalid --since: %w", err)
				}
				since = t.UTC()
			}
			tailCtx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt)
			defer stop()
			err = runLogsTail(tailCtx, a, LogsTailOptions{
				ProjectID: projectID,
				Since:     since,
				Interval:  pollInterval,
			})
			if tailCtx.Err() != nil && errors.Is(err, context.Canceled) {
				return nil
			}
			return err
		},
	}
	cmd.Flags().String("interval", "2s", "Poll interval")
	cmd.Flags().String("since", "", "Start time (RFC3339 or duration like 5m, 1h); default: now")
	return cmd
}

// parseTimeFlag parses a time flag value as either RFC3339 or a relative
// duration (e.g. "1h", "30m"). now is used as the reference for durations.
func parseTimeFlag(value string, now time.Time) (time.Time, error) {
	if t, err := time.Parse(time.RFC3339, value); err == nil {
		return t, nil
	}
	if d, err := time.ParseDuration(value); err == nil {
		return now.UTC().Add(-d), nil
	}
	return time.Time{}, fmt.Errorf("must be RFC3339 or a duration (e.g. 1h, 30m): %s", value)
}

func newLogsSearchCmd(a *app.App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "search <query>",
		Short: "Search project logs",
		Long:  "Search project logs for entries matching <query>. The query is a substring match against the log message. Use --since and --until to constrain the window; both accept RFC3339 or a duration like 1h.",
		Example: `  agiler logs search "panic"
  agiler logs search --since 24h "timeout"
  agiler logs search --limit 50 "500"`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			projectID, err := requireProjectID(a)
			if err != nil {
				return err
			}
			q := api.LogsQuery{Query: args[0]}

			if v, _ := cmd.Flags().GetInt("limit"); v > 0 {
				q.Limit = v
			}
			if cmd.Flags().Changed("since") {
				v, _ := cmd.Flags().GetString("since")
				t, err := parseTimeFlag(v, a.Clock.Now())
				if err != nil {
					return fmt.Errorf("invalid --since: %w", err)
				}
				q.Since = t
			}
			if cmd.Flags().Changed("until") {
				v, _ := cmd.Flags().GetString("until")
				t, err := parseTimeFlag(v, a.Clock.Now())
				if err != nil {
					return fmt.Errorf("invalid --until: %w", err)
				}
				q.Until = t
			}
			return runLogsQuery(cmd.Context(), a, projectID, q)
		},
	}
	cmd.Flags().Int("limit", 0, "Maximum entries returned (0 = server default)")
	cmd.Flags().String("since", "", "Start time (RFC3339 or duration like 1h, 24h)")
	cmd.Flags().String("until", "", "End time (RFC3339 or duration like 1h, 24h)")
	return cmd
}

func runLogsQuery(ctx context.Context, a *app.App, projectID string, q api.LogsQuery) error {
	if a.Output.IsStructured() {
		result, err := a.API.GetProjectLogs(ctx, projectID, q)
		if err != nil {
			return err
		}
		renderLogsList(a.Output, result)
		return nil
	}

	printed := false
	remaining := q.Limit
	seenCursors := map[string]struct{}{}
	if q.Cursor != "" {
		seenCursors[q.Cursor] = struct{}{}
	}
	for {
		pageQuery := q
		if q.Limit > 0 {
			if remaining <= 0 {
				break
			}
			pageSize := 1000
			if q.PageSize > 0 {
				pageSize = min(q.PageSize, 1000)
			}
			pageQuery.PageSize = min(remaining, pageSize)
		}
		page, err := a.API.GetProjectLogsPage(ctx, projectID, pageQuery)
		if err != nil {
			return err
		}
		if page == nil {
			break
		}
		items := page.Items
		if q.Limit > 0 && len(items) > remaining {
			items = items[:remaining]
		}
		if len(items) > 0 {
			renderLogsList(a.Output, items)
			printed = true
		}
		if q.Limit > 0 {
			remaining -= len(items)
			if remaining <= 0 {
				break
			}
		}
		if page.NextCursor == "" {
			break
		}
		if _, seen := seenCursors[page.NextCursor]; seen {
			break
		}
		seenCursors[page.NextCursor] = struct{}{}
		q.Cursor = page.NextCursor
	}
	if !printed {
		renderLogsList(a.Output, nil)
	}
	return nil
}

// --- Renderers ---

func renderProjectsList(w *output.Writer, ps []api.Project) {
	if w.IsStructured() {
		w.Structured(ps)
		return
	}
	if len(ps) == 0 {
		w.Text("No projects found.")
		return
	}
	rows := make([][]string, len(ps))
	for i, p := range ps {
		rows[i] = []string{p.Id.String(), p.Name, p.Status, p.Region, p.Runtime, p.WorkspaceId.String()}
	}
	w.Table([]string{"ID", "NAME", "STATUS", "REGION", "RUNTIME", "WORKSPACE"}, rows)
}

func renderProjectDetail(w *output.Writer, p api.ProjectDetail) error {
	if w.IsTabular() {
		return tabularUnsupportedErr(w)
	}
	if w.IsStructured() {
		w.Structured(p)
		return nil
	}
	if w.IsQuiet() {
		w.Text("%s", p.Id)
		return nil
	}
	w.Text("%s %s", w.OutColor.Dim("ID:       "), p.Id)
	w.Text("%s %s", w.OutColor.Dim("Name:     "), p.Name)
	w.Text("%s %s", w.OutColor.Dim("Status:   "), p.Status)
	w.Text("%s %s", w.OutColor.Dim("Active:   "), output.YesNo(p.Active))
	w.Text("%s %s", w.OutColor.Dim("Region:   "), p.Region)
	w.Text("%s %s", w.OutColor.Dim("Runtime:  "), p.Runtime)
	w.Text("%s %s", w.OutColor.Dim("Workspace:"), p.WorkspaceId)
	w.Text("%s %s", w.OutColor.Dim("Created:  "), p.CreatedAt.Format(time.RFC3339))
	w.Text("%s %s", w.OutColor.Dim("Updated:  "), p.UpdatedAt.Format(time.RFC3339))
	return nil
}

func renderUsageList(w *output.Writer, us []api.Usage) {
	if w.IsStructured() {
		w.Structured(us)
		return
	}
	if len(us) == 0 {
		w.Text("No usage data.")
		return
	}
	rows := make([][]string, len(us))
	for i, u := range us {
		rows[i] = []string{
			u.EventsAt.Format(time.RFC3339),
			fmt.Sprintf("%d", u.RequestsTotal),
			fmt.Sprintf("%d", u.Responses2xx),
			fmt.Sprintf("%d", u.Responses4xx),
			fmt.Sprintf("%d", u.Responses5xx),
			fmt.Sprintf("%d", u.DurationAverage),
			fmt.Sprintf("%d", u.DatatransferOut),
		}
	}
	w.Table(
		[]string{"DATE", "REQUESTS", "2XX", "4XX", "5XX", "AVG DURATION", "DATA OUT (MB)"},
		rows,
	)
}

func renderLogsList(w *output.Writer, ls []api.LogEntry) {
	if w.IsStructured() {
		w.Structured(ls)
		return
	}
	if len(ls) == 0 {
		w.Text("No logs found.")
		return
	}
	for _, l := range ls {
		w.Text("[%s] %s: %s", l.Timestamp.Format(time.RFC3339), l.Priority, l.Message)
	}
}

// tabularUnsupportedErr returns the standard error for callers that don't
// support tabular row/column layouts (csv/tsv) — typically detail or
// heterogeneous-data commands.
func tabularUnsupportedErr(w *output.Writer) error {
	return fmt.Errorf("--format=%s requires list output; use --format=json or --format=yaml", w.Format)
}
