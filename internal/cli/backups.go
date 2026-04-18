package cli

import (
	"fmt"
	"io"
	"os"

	"github.com/agilercloud/cli/internal/api"
	"github.com/agilercloud/cli/internal/app"
	"github.com/agilercloud/cli/internal/output"
	"github.com/spf13/cobra"
)

func newBackupsCmd(a *app.App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "backups",
		Short: "Manage project backups",
	}

	cmd.AddCommand(&cobra.Command{
		Use:   "list <project>",
		Short: "List project backups",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			var result api.BackupsResponse
			if err := a.API.DoJSON(cmd.Context(), "GET", fmt.Sprintf("/v1/projects/%s/backups", args[0]), nil, &result); err != nil {
				return err
			}
			renderBackupsList(a.Output, result)
			return nil
		},
	})

	cmd.AddCommand(&cobra.Command{
		Use:   "create <project>",
		Short: "Create a manual backup",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := a.API.DoJSONIdempotent(cmd.Context(), "POST", fmt.Sprintf("/v1/projects/%s/backups", args[0]), nil, nil); err != nil {
				return err
			}
			a.Output.Text("Backup created.")
			return nil
		},
	})

	cmd.AddCommand(&cobra.Command{
		Use:   "delete <project> <backup-id>",
		Short: "Delete a backup",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := a.API.DoJSON(cmd.Context(), "DELETE", fmt.Sprintf("/v1/projects/%s/backups/%s", args[0], args[1]), nil, nil); err != nil {
				return err
			}
			a.Output.Text("Backup deleted.")
			return nil
		},
	})

	cmd.AddCommand(&cobra.Command{
		Use:   "restore <project> <backup-id>",
		Short: "Restore a backup",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := a.API.DoJSONIdempotent(cmd.Context(), "POST", fmt.Sprintf("/v1/projects/%s/backups/%s/restore", args[0], args[1]), nil, nil); err != nil {
				return err
			}
			a.Output.Text("Backup restore initiated.")
			return nil
		},
	})

	dl := &cobra.Command{
		Use:   "download <project> <backup-id>",
		Short: "Download a backup",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			dlType, _ := cmd.Flags().GetString("type")
			if dlType != "storage" && dlType != "database" {
				return fmt.Errorf("--type must be 'storage' or 'database'")
			}

			outputPath, _ := cmd.Flags().GetString("output")

			path := fmt.Sprintf("/v1/projects/%s/backups/%s/%s", args[0], args[1], dlType)
			resp, err := a.API.Do(cmd.Context(), "GET", path, nil)
			if err != nil {
				return err
			}
			defer resp.Body.Close()

			var dest io.Writer
			var toClose io.Closer
			if outputPath == "" || outputPath == "-" {
				dest = a.Out
			} else {
				f, err := os.Create(outputPath)
				if err != nil {
					return fmt.Errorf("create output file: %w", err)
				}
				dest = f
				toClose = f
			}

			n, err := io.Copy(dest, resp.Body)
			if toClose != nil {
				toClose.Close()
			}
			if err != nil {
				return fmt.Errorf("write file: %w", err)
			}

			if outputPath != "" && outputPath != "-" {
				a.Output.Stderr("Downloaded %d bytes to %s", n, outputPath)
			}
			return nil
		},
	}
	dl.Flags().String("type", "", "Download type: 'storage' or 'database' (required)")
	dl.Flags().StringP("output", "o", "", "Output file path (default: stdout)")
	dl.MarkFlagRequired("type")
	cmd.AddCommand(dl)

	return cmd
}

func renderBackupsList(w *output.Writer, result api.BackupsResponse) {
	if w.IsJSON() {
		w.JSON(result)
		return
	}
	if len(result.Data) == 0 {
		w.Text("No backups found.")
		return
	}
	rows := make([][]string, len(result.Data))
	for i, b := range result.Data {
		rows[i] = []string{
			b.ID,
			b.Status,
			b.CreatedAt,
			fmt.Sprintf("%t", b.Automatic),
			fmt.Sprintf("%d", b.Size),
		}
	}
	w.Table([]string{"ID", "STATUS", "CREATED", "AUTO", "SIZE (MB)"}, rows)
	if !w.IsQuiet() {
		w.Text("\nBackup schedule: every %d hours, retain %d", result.Frequency, result.Retention)
	}
}
