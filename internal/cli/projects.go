package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"os/signal"
	"time"

	"github.com/agilercloud/cli/internal/api"
	"github.com/agilercloud/cli/internal/app"
	"github.com/agilercloud/cli/internal/output"
	"github.com/spf13/cobra"
)

func newProjectsCmd(a *app.App) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "projects",
		Aliases: []string{"project", "p"},
		Short:   "Manage projects",
	}

	cmd.AddCommand(newProjectsListCmd(a))
	cmd.AddCommand(newProjectsGetCmd(a))
	cmd.AddCommand(newProjectsCreateCmd(a))
	cmd.AddCommand(newProjectsUpdateCmd(a))
	cmd.AddCommand(newProjectsDeleteCmd(a))
	cmd.AddCommand(newVariablesCmd(a))
	cmd.AddCommand(newDomainsCmd(a))
	cmd.AddCommand(newProjectRulesCmd(a))
	cmd.AddCommand(newFilesCmd(a))
	cmd.AddCommand(newBackupsCmd(a))
	cmd.AddCommand(newSQLCmd(a))
	cmd.AddCommand(newUsageCmd(a))
	cmd.AddCommand(newLogsCmd(a))

	return cmd
}

func newProjectsListCmd(a *app.App) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List all projects",
		RunE: func(cmd *cobra.Command, args []string) error {
			var result []api.Project
			if err := a.API.DoJSON(cmd.Context(), "GET", "/v1/projects", nil, &result); err != nil {
				return err
			}
			renderProjectsList(a.Output, result)
			return nil
		},
	}
}

func newProjectsGetCmd(a *app.App) *cobra.Command {
	return &cobra.Command{
		Use:   "get <project>",
		Short: "Get project details",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			expand := "variables"
			var result api.Project
			if err := a.API.DoJSON(cmd.Context(), "GET", fmt.Sprintf("/v1/projects/%s?expand=%s", args[0], expand), nil, &result); err != nil {
				return err
			}
			renderProjectDetail(a.Output, result)
			return nil
		},
	}
}

func newProjectsCreateCmd(a *app.App) *cobra.Command {
	var name, region, runtime string
	var instance int

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a new project",
		RunE: func(cmd *cobra.Command, args []string) error {
			body := map[string]any{
				"name":     name,
				"region":   region,
				"runtime":  runtime,
				"instance": instance,
			}
			data, _ := json.Marshal(body)

			if err := a.API.DoJSONIdempotent(cmd.Context(), "POST", "/v1/projects", bytes.NewReader(data), nil); err != nil {
				return err
			}
			a.Output.Text("Project created.")
			return nil
		},
	}
	cmd.Flags().StringVar(&name, "name", "", "Project name (required)")
	cmd.Flags().StringVar(&region, "region", "", "Region ID (required)")
	cmd.Flags().StringVar(&runtime, "runtime", "", "Runtime ID (required)")
	cmd.Flags().IntVar(&instance, "instance", 0, "Instance type")
	cmd.MarkFlagRequired("name")
	cmd.MarkFlagRequired("region")
	cmd.MarkFlagRequired("runtime")
	return cmd
}

func newProjectsUpdateCmd(a *app.App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "update <project>",
		Short: "Update a project",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			body := map[string]any{}

			if cmd.Flags().Changed("name") {
				v, _ := cmd.Flags().GetString("name")
				body["name"] = v
			}
			if cmd.Flags().Changed("active") {
				v, _ := cmd.Flags().GetBool("active")
				body["active"] = v
			}
			if cmd.Flags().Changed("runtime") {
				v, _ := cmd.Flags().GetString("runtime")
				body["runtime"] = v
			}
			if cmd.Flags().Changed("instance") {
				v, _ := cmd.Flags().GetInt("instance")
				body["instance"] = v
			}

			if len(body) == 0 {
				return fmt.Errorf("no flags provided; use --name, --active, --runtime, or --instance")
			}

			data, _ := json.Marshal(body)
			if err := a.API.DoJSON(cmd.Context(), "PATCH", fmt.Sprintf("/v1/projects/%s", args[0]), bytes.NewReader(data), nil); err != nil {
				return err
			}
			a.Output.Text("Project updated.")
			return nil
		},
	}
	cmd.Flags().String("name", "", "Project name")
	cmd.Flags().Bool("active", false, "Active state")
	cmd.Flags().String("runtime", "", "Runtime ID")
	cmd.Flags().Int("instance", 0, "Instance type")
	return cmd
}

func newProjectsDeleteCmd(a *app.App) *cobra.Command {
	return &cobra.Command{
		Use:   "delete <project>",
		Short: "Delete a project",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := a.API.DoJSON(cmd.Context(), "DELETE", fmt.Sprintf("/v1/projects/%s", args[0]), nil, nil); err != nil {
				return err
			}
			a.Output.Text("Project deleted.")
			return nil
		},
	}
}

func newUsageCmd(a *app.App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "usage <project>",
		Short: "Get project usage statistics",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			limit, _ := cmd.Flags().GetString("limit")
			if limit == "" {
				limit = "7"
			}
			var result []api.UsageRecord
			if err := a.API.DoJSON(cmd.Context(), "GET", fmt.Sprintf("/v1/projects/%s/usage?limit=%s", args[0], limit), nil, &result); err != nil {
				return err
			}
			renderUsageList(a.Output, result)
			return nil
		},
	}
	cmd.Flags().String("limit", "7", "Number of data points")
	return cmd
}

func newLogsCmd(a *app.App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "logs <project>",
		Short: "Get project logs",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			limit, _ := cmd.Flags().GetString("limit")
			if limit == "" {
				limit = "50"
			}
			var result []api.LogEntry
			if err := a.API.DoJSON(cmd.Context(), "GET", fmt.Sprintf("/v1/projects/%s/logs?limit=%s", args[0], limit), nil, &result); err != nil {
				return err
			}
			renderLogsList(a.Output, result)
			return nil
		},
	}
	cmd.Flags().String("limit", "50", "Number of log entries")
	cmd.AddCommand(newLogsTailCmd(a))
	cmd.AddCommand(newLogsSearchCmd(a))
	return cmd
}

func newLogsTailCmd(a *app.App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "tail <project>",
		Short: "Stream project logs in real-time",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			interval, _ := cmd.Flags().GetString("interval")
			pollInterval, err := time.ParseDuration(interval)
			if err != nil {
				return fmt.Errorf("invalid interval: %w", err)
			}

			since := a.Clock.Now().UTC()
			seen := map[string]struct{}{}

			sigCh := make(chan os.Signal, 1)
			signal.Notify(sigCh, os.Interrupt)
			defer signal.Stop(sigCh)

			for {
				var result []api.LogEntry
				path := fmt.Sprintf("/v1/projects/%s/logs?since=%s&limit=1000",
					args[0], url.QueryEscape(since.Format(time.RFC3339Nano)))
				if err := a.API.DoJSON(cmd.Context(), "GET", path, nil, &result); err != nil {
					return err
				}

				for _, l := range result {
					key := l.RequestID + l.Message
					if _, dup := seen[key]; dup {
						continue
					}
					seen[key] = struct{}{}

					if a.Output.IsJSON() {
						a.Output.JSON(l)
					} else {
						a.Output.Text("[%s] %s: %s", l.Timestamp, l.Priority, l.Message)
					}

					if t, err := time.Parse(time.RFC3339Nano, l.Timestamp); err == nil {
						if t.After(since) {
							since = t.Add(time.Millisecond)
						}
					}
				}

				if len(seen) > 5000 {
					seen = map[string]struct{}{}
				}

				select {
				case <-sigCh:
					return nil
				case <-a.Clock.After(pollInterval):
				}
			}
		},
	}
	cmd.Flags().String("interval", "2s", "Poll interval")
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
		Use:   "search <project> <query>",
		Short: "Search project logs",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			limit, _ := cmd.Flags().GetString("limit")
			if limit == "" {
				limit = "100"
			}

			params := url.Values{}
			params.Set("q", args[1])
			params.Set("limit", limit)

			if cmd.Flags().Changed("since") {
				v, _ := cmd.Flags().GetString("since")
				t, err := parseTimeFlag(v, a.Clock.Now())
				if err != nil {
					return fmt.Errorf("invalid --since: %w", err)
				}
				params.Set("since", t.Format(time.RFC3339Nano))
			}
			if cmd.Flags().Changed("until") {
				v, _ := cmd.Flags().GetString("until")
				t, err := parseTimeFlag(v, a.Clock.Now())
				if err != nil {
					return fmt.Errorf("invalid --until: %w", err)
				}
				params.Set("until", t.Format(time.RFC3339Nano))
			}

			var result []api.LogEntry
			path := fmt.Sprintf("/v1/projects/%s/logs?%s", args[0], params.Encode())
			if err := a.API.DoJSON(cmd.Context(), "GET", path, nil, &result); err != nil {
				return err
			}
			renderLogsList(a.Output, result)
			return nil
		},
	}
	cmd.Flags().String("limit", "100", "Maximum number of results")
	cmd.Flags().String("since", "", "Start time (RFC3339 or duration like 1h, 24h)")
	cmd.Flags().String("until", "", "End time (RFC3339 or duration like 1h, 24h)")
	return cmd
}

// --- Renderers ---

func renderProjectsList(w *output.Writer, ps []api.Project) {
	if w.IsJSON() {
		w.JSON(ps)
		return
	}
	if len(ps) == 0 {
		w.Text("No projects found.")
		return
	}
	rows := make([][]string, len(ps))
	for i, p := range ps {
		rows[i] = []string{p.ID, p.Name, p.Status, p.Region, p.Runtime}
	}
	w.Table([]string{"ID", "NAME", "STATUS", "REGION", "RUNTIME"}, rows)
}

func renderProjectDetail(w *output.Writer, p api.Project) {
	if w.IsJSON() {
		w.JSON(p)
		return
	}
	if w.IsQuiet() {
		w.Text("%s", p.ID)
		return
	}
	w.Text("ID:        %s", p.ID)
	w.Text("Name:      %s", p.Name)
	w.Text("Status:    %s", p.Status)
	w.Text("Active:    %t", p.Active)
	w.Text("Region:    %s", p.Region)
	w.Text("Runtime:   %s", p.Runtime)
	w.Text("Instance:  %d", p.Instance)
	w.Text("Created:   %s", p.CreatedAt)
	w.Text("Updated:   %s", p.UpdatedAt)

	if len(p.Domains) > 0 {
		w.Text("\nDomains:")
		for _, d := range p.Domains {
			w.Text("  %s (%s)", d.Name, d.ID)
		}
	}
}

func renderUsageList(w *output.Writer, us []api.UsageRecord) {
	if w.IsJSON() {
		w.JSON(us)
		return
	}
	if len(us) == 0 {
		w.Text("No usage data.")
		return
	}
	rows := make([][]string, len(us))
	for i, u := range us {
		rows[i] = []string{
			u.EventsAt,
			fmt.Sprintf("%d", u.RequestsTotal),
			fmt.Sprintf("%d", u.Responses2xx),
			fmt.Sprintf("%d", u.Responses4xx),
			fmt.Sprintf("%d", u.Responses5xx),
			fmt.Sprintf("%v", u.DurationAverage),
			fmt.Sprintf("%v", u.DatatransferOut),
		}
	}
	w.Table(
		[]string{"DATE", "REQUESTS", "2XX", "4XX", "5XX", "AVG DURATION", "DATA OUT (MB)"},
		rows,
	)
}

func renderLogsList(w *output.Writer, ls []api.LogEntry) {
	if w.IsJSON() {
		w.JSON(ls)
		return
	}
	if len(ls) == 0 {
		w.Text("No logs found.")
		return
	}
	for _, l := range ls {
		w.Text("[%s] %s: %s", l.Timestamp, l.Priority, l.Message)
	}
}
