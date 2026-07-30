package cli

import (
	"context"
	"fmt"
	"io"
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
			return runBackupDownloadCommand(cmd, a, args[0], api.BackupDatabase)
		},
	}
	dlDB.Flags().StringP("output", "o", "", "Output file path (default: stdout)")
	dlDB.Flags().Bool("progress", false, "Show a streaming progress indicator on stderr")

	dlStorage := &cobra.Command{
		Use:   "storage <backup-id>",
		Short: "Download the object-storage snapshot from a backup",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runBackupDownloadCommand(cmd, a, args[0], api.BackupStorage)
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

// BackupDownloadOptions contains the parsed values for one artifact download.
type BackupDownloadOptions struct {
	ProjectID    string
	BackupID     string
	Kind         api.BackupArtifact
	OutputPath   string
	ShowProgress bool
}

func runBackupDownloadCommand(cmd *cobra.Command, a *app.App, backupID string, kind api.BackupArtifact) error {
	projectID, err := requireProjectID(a)
	if err != nil {
		return err
	}
	outputPath, _ := cmd.Flags().GetString("output")
	showProgress, _ := cmd.Flags().GetBool("progress")
	return runBackupDownload(cmd.Context(), a, BackupDownloadOptions{
		ProjectID:    projectID,
		BackupID:     backupID,
		Kind:         kind,
		OutputPath:   outputPath,
		ShowProgress: showProgress,
	})
}

func runBackupDownload(ctx context.Context, a *app.App, opts BackupDownloadOptions) error {
	resp, err := a.API.DownloadBackup(ctx, opts.ProjectID, opts.BackupID, opts.Kind)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	if opts.OutputPath == "" || opts.OutputPath == "-" {
		if _, err := io.Copy(a.Out, resp.Body); err != nil {
			return fmt.Errorf("write file: %w", err)
		}
		return nil
	}

	body := io.Reader(resp.Body)
	var prog *output.ProgressReader
	if opts.ShowProgress && a.Output.ErrColor.Enabled() {
		prog = output.NewProgressReader(resp.Body, a.Err, filepath.Base(opts.OutputPath), resp.ContentLength, a.Output.ErrColor)
		body = prog
	}

	n, err := writeStreamAtomic(opts.OutputPath, body)
	if prog != nil {
		prog.Finish(err == nil)
	}
	if err != nil {
		return err
	}
	if prog == nil {
		a.Output.Stderr("Downloaded %d bytes to %s", n, opts.OutputPath)
	}
	return nil
}
