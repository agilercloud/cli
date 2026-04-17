package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"os/signal"
	"time"

	"github.com/spf13/cobra"
)

var projectsCmd = &cobra.Command{
	Use:     "projects",
	Aliases: []string{"project", "p"},
	Short:   "Manage projects",
}

func init() {
	projectsCmd.AddCommand(projectsListCmd)
	projectsCmd.AddCommand(projectsGetCmd)
	projectsCmd.AddCommand(projectsCreateCmd)
	projectsCmd.AddCommand(projectsUpdateCmd)
	projectsCmd.AddCommand(projectsDeleteCmd)
	projectsCmd.AddCommand(variablesCmd)
	projectsCmd.AddCommand(domainsCmd)
	projectsCmd.AddCommand(projectRulesCmd)
	projectsCmd.AddCommand(filesCmd)
	projectsCmd.AddCommand(backupsCmd)
	projectsCmd.AddCommand(sqlCmd)
	projectsCmd.AddCommand(usageCmd)
	projectsCmd.AddCommand(logsCmd)
}

var projectsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all projects",
	RunE: func(cmd *cobra.Command, args []string) error {
		var result []map[string]any
		if err := apiClient.DoJSON("GET", "/v1/projects", nil, &result); err != nil {
			return err
		}

		if outputJSON {
			printJSON(result)
			return nil
		}

		if len(result) == 0 {
			fmt.Println("No projects found.")
			return nil
		}

		if outputQuiet {
			for _, p := range result {
				fmt.Println(p["id"])
			}
			return nil
		}

		w := newTabWriter()
		fmt.Fprintln(w, "ID\tNAME\tSTATUS\tREGION\tRUNTIME")
		for _, p := range result {
			fmt.Fprintf(w, "%v\t%v\t%v\t%v\t%v\n",
				p["id"], p["name"], p["status"], p["region"], p["runtime"])
		}
		w.Flush()
		return nil
	},
}

var projectsGetCmd = &cobra.Command{
	Use:   "get <project>",
	Short: "Get project details",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		expand := "variables"
		var result map[string]any
		if err := apiClient.DoJSON("GET", fmt.Sprintf("/v1/projects/%s?expand=%s", args[0], expand), nil, &result); err != nil {
			return err
		}

		if outputJSON {
			printJSON(result)
			return nil
		}

		if outputQuiet {
			fmt.Println(result["id"])
			return nil
		}

		fmt.Printf("ID:        %v\n", result["id"])
		fmt.Printf("Name:      %v\n", result["name"])
		fmt.Printf("Status:    %v\n", result["status"])
		fmt.Printf("Active:    %v\n", result["active"])
		fmt.Printf("Region:    %v\n", result["region"])
		fmt.Printf("Runtime:   %v\n", result["runtime"])
		fmt.Printf("Instance:  %v\n", result["instance"])
		fmt.Printf("Created:   %v\n", result["created_at"])
		fmt.Printf("Updated:   %v\n", result["updated_at"])

		if domains, ok := result["domains"].([]any); ok && len(domains) > 0 {
			fmt.Println("\nDomains:")
			for _, d := range domains {
				if dm, ok := d.(map[string]any); ok {
					fmt.Printf("  %v (%v)\n", dm["name"], dm["id"])
				}
			}
		}

		return nil
	},
}

var (
	createName     string
	createRegion   string
	createRuntime  string
	createInstance int
)

var projectsCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a new project",
	RunE: func(cmd *cobra.Command, args []string) error {
		body := map[string]any{
			"name":     createName,
			"region":   createRegion,
			"runtime":  createRuntime,
			"instance": createInstance,
		}
		data, _ := json.Marshal(body)

		if err := apiClient.DoJSONWithIdempotency("POST", "/v1/projects", bytes.NewReader(data), nil); err != nil {
			return err
		}

		fmt.Println("Project created.")
		return nil
	},
}

func init() {
	projectsCreateCmd.Flags().StringVar(&createName, "name", "", "Project name (required)")
	projectsCreateCmd.Flags().StringVar(&createRegion, "region", "", "Region ID (required)")
	projectsCreateCmd.Flags().StringVar(&createRuntime, "runtime", "", "Runtime ID (required)")
	projectsCreateCmd.Flags().IntVar(&createInstance, "instance", 0, "Instance type")
	projectsCreateCmd.MarkFlagRequired("name")
	projectsCreateCmd.MarkFlagRequired("region")
	projectsCreateCmd.MarkFlagRequired("runtime")
}

var projectsUpdateCmd = &cobra.Command{
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
		if err := apiClient.DoJSON("PATCH", fmt.Sprintf("/v1/projects/%s", args[0]), bytes.NewReader(data), nil); err != nil {
			return err
		}

		fmt.Println("Project updated.")
		return nil
	},
}

func init() {
	projectsUpdateCmd.Flags().String("name", "", "Project name")
	projectsUpdateCmd.Flags().Bool("active", false, "Active state")
	projectsUpdateCmd.Flags().String("runtime", "", "Runtime ID")
	projectsUpdateCmd.Flags().Int("instance", 0, "Instance type")
}

var projectsDeleteCmd = &cobra.Command{
	Use:   "delete <project>",
	Short: "Delete a project",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := apiClient.DoJSON("DELETE", fmt.Sprintf("/v1/projects/%s", args[0]), nil, nil); err != nil {
			return err
		}

		fmt.Println("Project deleted.")
		return nil
	},
}

// usage
var usageCmd = &cobra.Command{
	Use:   "usage <project>",
	Short: "Get project usage statistics",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		limit, _ := cmd.Flags().GetString("limit")
		if limit == "" {
			limit = "7"
		}

		var result []map[string]any
		if err := apiClient.DoJSON("GET", fmt.Sprintf("/v1/projects/%s/usage?limit=%s", args[0], limit), nil, &result); err != nil {
			return err
		}

		if outputJSON {
			printJSON(result)
			return nil
		}

		if len(result) == 0 {
			fmt.Println("No usage data.")
			return nil
		}

		w := newTabWriter()
		fmt.Fprintln(w, "DATE\tREQUESTS\t2XX\t4XX\t5XX\tAVG DURATION\tDATA OUT (MB)")
		for _, u := range result {
			fmt.Fprintf(w, "%v\t%v\t%v\t%v\t%v\t%v\t%v\n",
				u["events_at"], u["requests_total"],
				u["responses_2xx"], u["responses_4xx"], u["responses_5xx"],
				u["duration_average"], u["datatransfer_out"])
		}
		w.Flush()
		return nil
	},
}

func init() {
	usageCmd.Flags().String("limit", "7", "Number of data points")
}

// logs
var logsCmd = &cobra.Command{
	Use:   "logs <project>",
	Short: "Get project logs",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		limit, _ := cmd.Flags().GetString("limit")
		if limit == "" {
			limit = "50"
		}

		var result []map[string]any
		if err := apiClient.DoJSON("GET", fmt.Sprintf("/v1/projects/%s/logs?limit=%s", args[0], limit), nil, &result); err != nil {
			return err
		}

		if outputJSON {
			printJSON(result)
			return nil
		}

		if len(result) == 0 {
			fmt.Println("No logs found.")
			return nil
		}

		for _, l := range result {
			fmt.Printf("[%v] %v: %v\n", l["timestamp"], l["priority"], l["message"])
		}
		return nil
	},
}

func init() {
	logsCmd.Flags().String("limit", "50", "Number of log entries")
	logsCmd.AddCommand(logsTailCmd)
	logsCmd.AddCommand(logsSearchCmd)
}

// logsTailCmd streams project logs in real-time by polling.
var logsTailCmd = &cobra.Command{
	Use:   "tail <project>",
	Short: "Stream project logs in real-time",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		interval, _ := cmd.Flags().GetString("interval")
		pollInterval, err := time.ParseDuration(interval)
		if err != nil {
			return fmt.Errorf("invalid interval: %w", err)
		}

		since := time.Now().UTC()
		seen := map[string]struct{}{}

		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, os.Interrupt)
		defer signal.Stop(sigCh)

		for {
			var result []map[string]any
			path := fmt.Sprintf("/v1/projects/%s/logs?since=%s&limit=1000",
				args[0], url.QueryEscape(since.Format(time.RFC3339Nano)))
			if err := apiClient.DoJSON("GET", path, nil, &result); err != nil {
				return err
			}

			for _, l := range result {
				rid, _ := l["request_id"].(string)
				msg, _ := l["message"].(string)
				key := rid + msg
				if _, dup := seen[key]; dup {
					continue
				}
				seen[key] = struct{}{}

				if outputJSON {
					printJSON(l)
				} else {
					fmt.Printf("[%v] %v: %v\n", l["timestamp"], l["priority"], l["message"])
				}

				// advance since to latest event timestamp
				if ts, ok := l["timestamp"].(string); ok {
					if t, err := time.Parse(time.RFC3339Nano, ts); err == nil {
						if t.After(since) {
							since = t.Add(time.Millisecond)
						}
					}
				}
			}

			// cap seen set size
			if len(seen) > 5000 {
				seen = map[string]struct{}{}
			}

			select {
			case <-sigCh:
				return nil
			case <-time.After(pollInterval):
			}
		}
	},
}

func init() {
	logsTailCmd.Flags().String("interval", "2s", "Poll interval")
}

// parseTimeFlag parses a time flag value as either RFC3339 or a relative duration (e.g. "1h", "30m").
func parseTimeFlag(value string) (time.Time, error) {
	if t, err := time.Parse(time.RFC3339, value); err == nil {
		return t, nil
	}
	if d, err := time.ParseDuration(value); err == nil {
		return time.Now().UTC().Add(-d), nil
	}
	return time.Time{}, fmt.Errorf("must be RFC3339 or a duration (e.g. 1h, 30m): %s", value)
}

// logsSearchCmd searches project logs.
var logsSearchCmd = &cobra.Command{
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
			t, err := parseTimeFlag(v)
			if err != nil {
				return fmt.Errorf("invalid --since: %w", err)
			}
			params.Set("since", t.Format(time.RFC3339Nano))
		}
		if cmd.Flags().Changed("until") {
			v, _ := cmd.Flags().GetString("until")
			t, err := parseTimeFlag(v)
			if err != nil {
				return fmt.Errorf("invalid --until: %w", err)
			}
			params.Set("until", t.Format(time.RFC3339Nano))
		}

		var result []map[string]any
		path := fmt.Sprintf("/v1/projects/%s/logs?%s", args[0], params.Encode())
		if err := apiClient.DoJSON("GET", path, nil, &result); err != nil {
			return err
		}

		if outputJSON {
			printJSON(result)
			return nil
		}

		if len(result) == 0 {
			fmt.Println("No logs found.")
			return nil
		}

		for _, l := range result {
			fmt.Printf("[%v] %v: %v\n", l["timestamp"], l["priority"], l["message"])
		}
		return nil
	},
}

func init() {
	logsSearchCmd.Flags().String("limit", "100", "Maximum number of results")
	logsSearchCmd.Flags().String("since", "", "Start time (RFC3339 or duration like 1h, 24h)")
	logsSearchCmd.Flags().String("until", "", "End time (RFC3339 or duration like 1h, 24h)")
}
