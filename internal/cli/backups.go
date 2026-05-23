package cli

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/agilercloud/cli/internal/api"
	"github.com/agilercloud/cli/internal/app"
	"github.com/agilercloud/cli/internal/output"
	"github.com/spf13/cobra"
)

func newBackupsCmd(a *app.App) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "backups",
		Aliases: []string{"backup"},
		Short:   "Manage project backups",
	}

	cmd.AddCommand(&cobra.Command{
		Use:   "list",
		Short: "List project backups",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			projectID, err := requireProjectID(a)
			if err != nil {
				return err
			}
			result, err := a.API.ListBackups(cmd.Context(), projectID)
			if err != nil {
				return err
			}
			renderBackupsList(a.Output, result)
			return nil
		},
	})

	cmd.AddCommand(&cobra.Command{
		Use:     "create",
		Short:   "Create a manual backup",
		Long:    "Create a manual backup of the configured project. Manual backups don't count against the retention policy's automatic-backup cap, but they are still subject to the retention window.",
		Example: `  agiler backups create`,
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			projectID, err := requireProjectID(a)
			if err != nil {
				return err
			}
			result, err := a.API.CreateBackup(cmd.Context(), projectID)
			if err != nil {
				return err
			}
			if a.Output.IsStructured() {
				a.Output.Structured(result)
			} else if a.Output.IsQuiet() {
				a.Output.Text("%s", result.Id)
			} else {
				a.Output.Text("Backup created: %s", result.Id)
			}
			return nil
		},
	})

	deleteCmd := &cobra.Command{
		Use:   "delete <backup-id>",
		Short: "Delete a backup",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			projectID, err := requireProjectID(a)
			if err != nil {
				return err
			}
			if err := confirmOrSkip(a, cmd, fmt.Sprintf("Delete backup %s? (y/N) ", args[0])); err != nil {
				return err
			}
			if err := a.API.DeleteBackup(cmd.Context(), projectID, args[0]); err != nil {
				return err
			}
			a.Output.Text("Backup deleted.")
			return nil
		},
	}
	addYesFlag(deleteCmd)
	cmd.AddCommand(deleteCmd)

	restoreCmd := &cobra.Command{
		Use:   "restore <backup-id>",
		Short: "Restore a backup",
		Long:  "Restore the configured project to the state captured by the given backup. This replaces the live database and object storage with the backup's contents. Use --drain-requests to wait for in-flight requests to finish before starting the restore. The command requires confirmation; pass --yes to skip.",
		Example: `  agiler backups restore 01HXY...
  agiler backups restore 01HXY... --drain-requests
  agiler backups restore 01HXY... --yes`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			projectID, err := requireProjectID(a)
			if err != nil {
				return err
			}
			drainRequests, _ := cmd.Flags().GetBool("drain-requests")
			var prompt string
			if drainRequests {
				prompt = fmt.Sprintf("Drain in-flight requests and restore backup %s to project %s? This is irreversible. (y/N) ", args[0], projectID)
			} else {
				prompt = fmt.Sprintf("Restore backup %s to project %s? This is irreversible. (y/N) ", args[0], projectID)
			}
			if err := confirmOrSkip(a, cmd, prompt); err != nil {
				return err
			}
			if err := a.API.RestoreBackup(cmd.Context(), projectID, args[0], drainRequests); err != nil {
				return err
			}
			a.Output.Text("Backup restore initiated.")
			return nil
		},
	}
	restoreCmd.Flags().Bool("drain-requests", false, "Wait for in-flight requests to drain before restoring")
	addYesFlag(restoreCmd)
	cmd.AddCommand(restoreCmd)

	dl := &cobra.Command{
		Use:   "download",
		Short: "Download a backup artifact (database dump or object storage)",
		Long:  "Download a backup artifact. A backup includes both a database dump and an object-storage snapshot; choose which to download via the database or storage subcommand.",
		Example: `  agiler backups download database 01HXY... -o backup.sql
  agiler backups download storage 01HXY... -o storage.tar.gz`,
	}

	dlDB := &cobra.Command{
		Use:   "database <backup-id>",
		Short: "Download the database dump from a backup",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runBackupDownload(cmd, a, args, api.BackupDatabase)
		},
	}
	dlDB.Flags().StringP("output", "o", "", "Output file path (default: stdout)")
	dlDB.Flags().Bool("progress", false, "Show a streaming progress indicator on stderr")

	dlStorage := &cobra.Command{
		Use:   "storage <backup-id>",
		Short: "Download the object-storage snapshot from a backup",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runBackupDownload(cmd, a, args, api.BackupStorage)
		},
	}
	dlStorage.Flags().StringP("output", "o", "", "Output file path (default: stdout)")
	dlStorage.Flags().Bool("progress", false, "Show a streaming progress indicator on stderr")

	dl.AddCommand(dlDB, dlStorage)
	cmd.AddCommand(dl)

	policy := &cobra.Command{
		Use:   "policy",
		Short: "Show backup policy",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			projectID, err := requireProjectID(a)
			if err != nil {
				return err
			}
			result, err := a.API.GetBackupPolicy(cmd.Context(), projectID)
			if err != nil {
				return err
			}
			renderBackupPolicy(a.Output, *result)
			return nil
		},
	}

	policySet := &cobra.Command{
		Use:   "set",
		Short: "Update backup policy (--frequency-days, --retention-days)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			projectID, err := requireProjectID(a)
			if err != nil {
				return err
			}
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
			policy, err := a.API.SetBackupPolicy(cmd.Context(), projectID, in)
			if err != nil {
				return err
			}
			renderBackupPolicy(a.Output, *policy)
			return nil
		},
	}
	policySet.Flags().Int("frequency-days", 0, "Backup frequency in days (0 disables)")
	policySet.Flags().Int("retention-days", 0, "Backup retention in days")
	policy.AddCommand(policySet)

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
			output.YesNo(b.Automatic),
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

func runBackupDownload(cmd *cobra.Command, a *app.App, args []string, kind api.BackupArtifact) error {
	projectID, err := requireProjectID(a)
	if err != nil {
		return err
	}
	outputPath, _ := cmd.Flags().GetString("output")
	showProgress, _ := cmd.Flags().GetBool("progress")
	resp, err := a.API.DownloadBackup(cmd.Context(), projectID, args[0], kind)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	if outputPath == "" || outputPath == "-" {
		if _, err := io.Copy(a.Out, resp.Body); err != nil {
			return fmt.Errorf("write file: %w", err)
		}
		return nil
	}

	body := io.Reader(resp.Body)
	var prog *output.ProgressReader
	if showProgress && a.Output.ErrColor.Enabled() {
		prog = output.NewProgressReader(resp.Body, a.Err, filepath.Base(outputPath), resp.ContentLength, a.Output.ErrColor)
		body = prog
	}

	n, err := writeStreamAtomic(outputPath, body)
	if prog != nil {
		prog.Finish(err == nil)
	}
	if err != nil {
		return err
	}
	if prog == nil {
		a.Output.Stderr("Downloaded %d bytes to %s", n, outputPath)
	}
	return nil
}

// writeStreamAtomic copies src into a sibling temp file of outputPath
// and renames it on success, leaving the existing file at outputPath
// untouched if the copy fails mid-stream. Returns the number of bytes
// written.
func writeStreamAtomic(outputPath string, src io.Reader) (int64, error) {
	tmp, err := os.CreateTemp(filepath.Dir(outputPath), ".agiler-download-*")
	if err != nil {
		return 0, fmt.Errorf("create temp file: %w", err)
	}
	tmpPath := tmp.Name()
	cleanup := true
	defer func() {
		_ = tmp.Close()
		if cleanup {
			_ = os.Remove(tmpPath)
		}
	}()

	n, err := io.Copy(tmp, src)
	if err != nil {
		return n, fmt.Errorf("write file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return n, fmt.Errorf("close temp: %w", err)
	}
	if err := os.Rename(tmpPath, outputPath); err != nil {
		return n, fmt.Errorf("finalize: %w", err)
	}
	cleanup = false
	return n, nil
}
