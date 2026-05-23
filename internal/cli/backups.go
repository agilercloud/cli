package cli

import (
	"fmt"
	"io"
	"os"
	"time"

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
			result, err := a.API.ListBackups(cmd.Context(), args[0])
			if err != nil {
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
			if _, err := a.API.CreateBackup(cmd.Context(), args[0]); err != nil {
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
			if err := a.API.DeleteBackup(cmd.Context(), args[0], args[1]); err != nil {
				return err
			}
			a.Output.Text("Backup deleted.")
			return nil
		},
	})

	restoreCmd := &cobra.Command{
		Use:   "restore <project> <backup-id>",
		Short: "Restore a backup",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			drainRequests, _ := cmd.Flags().GetBool("drain-requests")
			if err := a.API.RestoreBackup(cmd.Context(), args[0], args[1], drainRequests); err != nil {
				return err
			}
			a.Output.Text("Backup restore initiated.")
			return nil
		},
	}
	restoreCmd.Flags().Bool("drain-requests", false, "Wait for in-flight requests to drain before restoring")
	cmd.AddCommand(restoreCmd)

	dl := &cobra.Command{
		Use:   "download <project> <backup-id>",
		Short: "Download a backup",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			dlType, _ := cmd.Flags().GetString("type")
			var kind api.BackupArtifact
			switch dlType {
			case "database":
				kind = api.BackupDatabase
			case "storage":
				kind = api.BackupStorage
			default:
				return fmt.Errorf("--type must be 'storage' or 'database'")
			}

			outputPath, _ := cmd.Flags().GetString("output")
			resp, err := a.API.DownloadBackup(cmd.Context(), args[0], args[1], kind)
			if err != nil {
				return err
			}
			defer func() { _ = resp.Body.Close() }()

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
				_ = toClose.Close()
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
	_ = dl.MarkFlagRequired("type")
	cmd.AddCommand(dl)

	policy := &cobra.Command{
		Use:   "policy <project>",
		Short: "Show backup policy",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			result, err := a.API.GetBackupPolicy(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			renderBackupPolicy(a.Output, *result)
			return nil
		},
	}

	policy.AddCommand(&cobra.Command{
		Use:   "set <project>",
		Short: "Update backup policy (--frequency-days, --retention-days)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			var in api.UpdateBackupPolicy
			touched := false
			if cmd.Flags().Changed("frequency-days") {
				v, _ := cmd.Flags().GetInt("frequency-days")
				in.FrequencyDays = &v
				touched = true
			}
			if cmd.Flags().Changed("retention-days") {
				v, _ := cmd.Flags().GetInt("retention-days")
				in.RetentionDays = &v
				touched = true
			}
			if !touched {
				return fmt.Errorf("provide --frequency-days and/or --retention-days")
			}
			policy, err := a.API.SetBackupPolicy(cmd.Context(), args[0], in)
			if err != nil {
				return err
			}
			renderBackupPolicy(a.Output, *policy)
			return nil
		},
	})
	policy.PersistentFlags().Int("frequency-days", 0, "Backup frequency in days (0 disables)")
	policy.PersistentFlags().Int("retention-days", 0, "Backup retention in days")

	cmd.AddCommand(policy)

	return cmd
}

func renderBackupsList(w *output.Writer, backups []api.Backup) {
	if w.IsStructured() {
		w.Structured(backups)
		return
	}
	if len(backups) == 0 {
		w.Text("No backups found.")
		return
	}
	rows := make([][]string, len(backups))
	for i, b := range backups {
		size := ""
		if b.Size != nil {
			size = fmt.Sprintf("%d", *b.Size)
		}
		rows[i] = []string{
			b.Id.String(),
			b.Status,
			b.CreatedAt.Format(time.RFC3339),
			fmt.Sprintf("%t", b.Automatic),
			size,
		}
	}
	w.Table([]string{"ID", "STATUS", "CREATED", "AUTO", "SIZE (MB)"}, rows)
}

func renderBackupPolicy(w *output.Writer, p api.BackupPolicy) {
	if w.IsStructured() {
		w.Structured(p)
		return
	}
	w.Text("Frequency: every %d days", p.FrequencyDays)
	w.Text("Retention: %d days", p.RetentionDays)
}
