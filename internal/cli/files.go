package cli

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/agilercloud/cli/internal/api"
	"github.com/agilercloud/cli/internal/app"
	"github.com/agilercloud/cli/internal/fsx"
	"github.com/agilercloud/cli/internal/output"
	"github.com/spf13/cobra"
)

func newFilesCmd(a *app.App) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "files",
		Aliases: []string{"file"},
		Short:   "Manage project files",
	}

	cmd.AddCommand(newFilesListCmd(a))
	cmd.AddCommand(newFilesGetCmd(a))
	cmd.AddCommand(newFilesUploadCmd(a))
	cmd.AddCommand(newFilesDeleteCmd(a))
	cmd.AddCommand(newFilesMoveCmd(a))
	cmd.AddCommand(newFilesCopyCmd(a))

	return cmd
}

type syncStats struct {
	transferred int
	skipped     int
	errors      int
}

func (s *syncStats) print(w *output.Writer) {
	w.Stderr("%d transferred, %d skipped, %d errors", s.transferred, s.skipped, s.errors)
}

// shouldSkip reports whether the local file matches the remote in size
// and mtime, in which case the transfer can be skipped.
func shouldSkip(fs fsx.FS, localPath string, remoteSize int, remoteModifiedAt time.Time) bool {
	info, err := fs.Stat(localPath)
	if err != nil {
		return false
	}
	if info.Size() != int64(remoteSize) {
		return false
	}
	return info.ModTime().Unix() == remoteModifiedAt.Unix()
}

func remoteParentDir(remotePath string) string {
	remotePath = strings.TrimPrefix(remotePath, "/")
	dir := path.Dir(remotePath)
	if dir == "." {
		return ""
	}
	return dir
}

// --- Upload ---

func uploadSingleFile(ctx context.Context, client *api.Client, projectID, remotePath, localPath string, overwrite bool, prog *progressOptions) (err error) {
	f, err := os.Open(localPath)
	if err != nil {
		return fmt.Errorf("open file: %w", err)
	}
	defer func() { _ = f.Close() }()

	headers := map[string]string{}
	var size int64
	if fi, err := f.Stat(); err == nil {
		headers["Last-Modified"] = fi.ModTime().UTC().Format(http.TimeFormat)
		size = fi.Size()
	}
	if !overwrite {
		headers["If-None-Match"] = "*"
	}

	body := io.Reader(f)
	var pr *output.ProgressReader
	if prog != nil {
		pr = output.NewProgressReader(f, prog.w, filepath.Base(localPath), size, prog.color)
		body = pr
	}
	defer func() {
		if pr != nil {
			pr.Finish(err == nil)
		}
	}()

	err = client.PutProjectFile(ctx, projectID, remotePath, "application/octet-stream", body, headers)
	return err
}

func uploadDir(ctx context.Context, a *app.App, projectID, remoteBase, localDir string, force, overwrite bool, stats *syncStats) error {
	entries, err := os.ReadDir(localDir)
	if err != nil {
		return fmt.Errorf("read directory: %w", err)
	}

	var remoteMap map[string]api.File
	if !force {
		remoteEntries, err := a.API.ListProjectFiles(ctx, projectID, remoteBase)
		if err == nil {
			remoteMap = make(map[string]api.File, len(remoteEntries))
			for _, e := range remoteEntries {
				remoteMap[e.Name] = e
			}
		}
	}

	for _, entry := range entries {
		if entry.Type()&os.ModeSymlink != 0 {
			continue
		}

		localPath := filepath.Join(localDir, entry.Name())
		remotePath := path.Join(remoteBase, entry.Name())

		if entry.IsDir() {
			if err := uploadDir(ctx, a, projectID, remotePath, localPath, force, overwrite, stats); err != nil {
				return err
			}
			continue
		}

		if !force && remoteMap != nil {
			if re, ok := remoteMap[entry.Name()]; ok && !re.IsDir {
				if shouldSkip(a.FS, localPath, re.Size, re.ModifiedAt) {
					a.Output.Stderr("skip %s", remotePath)
					stats.skipped++
					continue
				}
			}
		}

		if err := uploadSingleFile(ctx, a.API, projectID, remotePath, localPath, overwrite, nil); err != nil {
			a.Output.Stderr("error %s: %v", remotePath, err)
			stats.errors++
			continue
		}
		a.Output.Stderr("upload %s", remotePath)
		stats.transferred++
	}
	return nil
}

func newFilesUploadCmd(a *app.App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "upload <remote-path> <local-path>",
		Short: "Upload a file or directory to a project",
		Long:  "Upload a file or directory to the configured project. Directory uploads are recursive and skip files that match the remote in size and mtime; use --force to disable the skip. By default, uploads fail if the remote destination already exists; pass --overwrite to replace it.",
		Example: `  agiler files upload /index.html ./index.html
  agiler files upload --overwrite /app.js ./build/app.js
  agiler files upload /static ./public`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			projectID, err := requireProjectID(a)
			if err != nil {
				return err
			}
			force, _ := cmd.Flags().GetBool("force")
			overwrite, _ := cmd.Flags().GetBool("overwrite")
			showProgress, _ := cmd.Flags().GetBool("progress")
			return runFilesUpload(cmd.Context(), a, FilesUploadOptions{
				ProjectID:    projectID,
				RemotePath:   args[0],
				LocalPath:    args[1],
				Force:        force,
				Overwrite:    overwrite,
				ShowProgress: showProgress,
			})
		},
	}
	cmd.Flags().BoolP("force", "f", false, "Force transfer even if file is unchanged")
	cmd.Flags().Bool("overwrite", false, "Overwrite the remote destination if it already exists (default: fail with 412 if exists)")
	cmd.Flags().Bool("progress", false, "Show a streaming progress indicator on stderr (single-file uploads only)")
	return cmd
}

// FilesUploadOptions contains the parsed inputs for a file or directory
// upload. The runner owns filesystem inspection, skip checks, and transfer.
type FilesUploadOptions struct {
	ProjectID    string
	RemotePath   string
	LocalPath    string
	Force        bool
	Overwrite    bool
	ShowProgress bool
}

func runFilesUpload(ctx context.Context, a *app.App, opts FilesUploadOptions) error {
	fi, err := os.Stat(opts.LocalPath)
	if err != nil {
		return fmt.Errorf("stat local path: %w", err)
	}

	if fi.IsDir() {
		stats := &syncStats{}
		if err := uploadDir(ctx, a, opts.ProjectID, opts.RemotePath, opts.LocalPath, opts.Force, opts.Overwrite, stats); err != nil {
			return err
		}
		stats.print(a.Output)
		return nil
	}

	if !opts.Force {
		parentDir := remoteParentDir(opts.RemotePath)
		remoteEntries, err := a.API.ListProjectFiles(ctx, opts.ProjectID, parentDir)
		if err == nil {
			baseName := path.Base(opts.RemotePath)
			for _, remoteEntry := range remoteEntries {
				if remoteEntry.Name == baseName && !remoteEntry.IsDir {
					if shouldSkip(a.FS, opts.LocalPath, remoteEntry.Size, remoteEntry.ModifiedAt) {
						a.Output.Stderr("skip (unchanged)")
						return nil
					}
					break
				}
			}
		}
	}

	var prog *progressOptions
	if opts.ShowProgress && a.Output.ErrColor.Enabled() {
		prog = &progressOptions{w: a.Err, color: a.Output.ErrColor}
	}
	if err := uploadSingleFile(ctx, a.API, opts.ProjectID, opts.RemotePath, opts.LocalPath, opts.Overwrite, prog); err != nil {
		return err
	}
	if prog == nil {
		a.Output.Text("File uploaded.")
	}
	return nil
}

// --- Download ---

func downloadSingleFile(ctx context.Context, a *app.App, projectID, remotePath, localPath string, showProgress bool) error {
	resp, err := a.API.GetProjectFile(ctx, projectID, remotePath)
	if err != nil {
		return err
	}
	return writeDownloadResponse(a, resp, downloadResponseOptions{
		OutputPath:   localPath,
		ShowProgress: showProgress,
		BeforeWrite: func() error {
			if err := os.MkdirAll(filepath.Dir(localPath), 0o755); err != nil {
				return fmt.Errorf("create parent directory: %w", err)
			}
			return nil
		},
		AfterWrite: func() {
			if lastMod := resp.Header.Get("Last-Modified"); lastMod != "" {
				if t, err := http.ParseTime(lastMod); err == nil {
					_ = os.Chtimes(localPath, t, t)
				}
			}
		},
	})
}

type progressOptions struct {
	w     io.Writer
	color output.Color
}

func downloadDir(ctx context.Context, a *app.App, projectID, remoteBase, localDir string, force bool, stats *syncStats) error {
	entries, err := a.API.ListProjectFiles(ctx, projectID, remoteBase)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(localDir, 0o755); err != nil {
		return fmt.Errorf("create directory: %w", err)
	}

	for _, entry := range entries {
		localPath := filepath.Join(localDir, entry.Name)
		remotePath := entry.Path

		if entry.IsDir {
			if err := downloadDir(ctx, a, projectID, remotePath, localPath, force, stats); err != nil {
				return err
			}
			continue
		}

		if !force && shouldSkip(a.FS, localPath, entry.Size, entry.ModifiedAt) {
			a.Output.Stderr("skip %s", remotePath)
			stats.skipped++
			continue
		}

		if err := downloadSingleFile(ctx, a, projectID, remotePath, localPath, false); err != nil {
			a.Output.Stderr("error %s: %v", remotePath, err)
			stats.errors++
			continue
		}
		stats.transferred++
	}
	return nil
}

func newFilesGetCmd(a *app.App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "get <path>",
		Short: "Download a file or directory from a project",
		Long:  "Download a file or directory from the configured project. With no -o flag, a single file streams to stdout; -o writes to a path. For directories, -o is required and downloads recursively, skipping files that match remote size and mtime.",
		Example: `  agiler files get /index.html
  agiler files get /index.html -o ./index.html
  agiler files get /static -o ./public`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			projectID, err := requireProjectID(a)
			if err != nil {
				return err
			}
			outputPath, _ := cmd.Flags().GetString("output")
			force, _ := cmd.Flags().GetBool("force")
			showProgress, _ := cmd.Flags().GetBool("progress")
			return runFilesGet(cmd.Context(), a, FilesGetOptions{
				ProjectID:    projectID,
				RemotePath:   args[0],
				OutputPath:   outputPath,
				Force:        force,
				ShowProgress: showProgress,
			})
		},
	}
	cmd.Flags().StringP("output", "o", "", "Output file or directory path (default: stdout)")
	cmd.Flags().BoolP("force", "f", false, "Force transfer even if file is unchanged")
	cmd.Flags().Bool("progress", false, "Show a streaming progress indicator on stderr (single-file downloads only)")
	return cmd
}

// FilesGetOptions contains the parsed inputs for a file or directory
// download. The runner owns remote probing, skip checks, and streaming.
type FilesGetOptions struct {
	ProjectID    string
	RemotePath   string
	OutputPath   string
	Force        bool
	ShowProgress bool
}

func runFilesGet(ctx context.Context, a *app.App, opts FilesGetOptions) error {
	entries, listErr := a.API.ListProjectFiles(ctx, opts.ProjectID, opts.RemotePath)
	isDir := listErr == nil && entries != nil

	if isDir {
		if opts.OutputPath == "" || opts.OutputPath == "-" {
			return fmt.Errorf("cannot download directory to stdout; use -o to specify output directory")
		}
		stats := &syncStats{}
		if err := downloadDir(ctx, a, opts.ProjectID, opts.RemotePath, opts.OutputPath, opts.Force, stats); err != nil {
			return err
		}
		stats.print(a.Output)
		return nil
	}

	if opts.OutputPath != "" && opts.OutputPath != "-" && !opts.Force {
		parentDir := remoteParentDir(opts.RemotePath)
		remoteEntries, err := a.API.ListProjectFiles(ctx, opts.ProjectID, parentDir)
		if err == nil {
			baseName := path.Base(opts.RemotePath)
			for _, remoteEntry := range remoteEntries {
				if remoteEntry.Name == baseName && !remoteEntry.IsDir {
					if shouldSkip(a.FS, opts.OutputPath, remoteEntry.Size, remoteEntry.ModifiedAt) {
						a.Output.Stderr("skip (unchanged)")
						return nil
					}
					break
				}
			}
		}
	}

	return downloadSingleFile(ctx, a, opts.ProjectID, opts.RemotePath, opts.OutputPath, opts.ShowProgress)
}

// --- List ---

func newFilesListCmd(a *app.App) *cobra.Command {
	return &cobra.Command{
		Use:   "list [path]",
		Short: "List files in a project directory",
		Args:  cobra.RangeArgs(0, 1),
		RunE: func(cmd *cobra.Command, args []string) error {
			projectID, err := requireProjectID(a)
			if err != nil {
				return err
			}
			var sub string
			if len(args) > 0 {
				sub = args[0]
			}
			result, err := a.API.ListProjectFiles(cmd.Context(), projectID, sub)
			if err != nil {
				return err
			}
			renderFilesList(a.Output, result)
			return nil
		},
	}
}

func newFilesDeleteCmd(a *app.App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "delete <path>",
		Short: "Delete a file from a project",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			projectID, err := requireProjectID(a)
			if err != nil {
				return err
			}
			if err := confirmOrSkip(a, cmd, fmt.Sprintf("Delete file %s? (y/N) ", args[0])); err != nil {
				return err
			}
			if err := a.API.DeleteProjectFile(cmd.Context(), projectID, args[0]); err != nil {
				return err
			}
			a.Output.Text("File deleted.")
			return nil
		},
	}
	addYesFlag(cmd)
	return cmd
}

// projectFileSource builds the canonical X-Move-Source / X-Copy-Source
// header value: "{path}" with path segments percent-encoded.
func projectFileSource(sourcePath string) string {
	return api.EncodeFilePath(sourcePath)
}

func newFilesMoveCmd(a *app.App) *cobra.Command {
	return newFilesTransferCmd(a, "move", "Move or rename a file", "X-Move-Source", "File moved.")
}

func newFilesCopyCmd(a *app.App) *cobra.Command {
	return newFilesTransferCmd(a, "copy", "Copy a file or directory", "X-Copy-Source", "File copied.")
}

func newFilesTransferCmd(a *app.App, name, short, sourceHeader, successText string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   name + " <source> <destination>",
		Short: short,
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			overwrite, _ := cmd.Flags().GetBool("overwrite")
			projectID, err := requireProjectID(a)
			if err != nil {
				return err
			}
			source := args[0]
			destination := args[1]

			headers := map[string]string{
				sourceHeader: projectFileSource(source),
			}
			if !overwrite {
				headers["If-None-Match"] = "*"
			}

			// contentType="" signals the move/copy mode: no body, no
			// Content-Type. PutProjectFile strips the Content-Type the
			// generated client would otherwise force.
			if err := a.API.PutProjectFile(cmd.Context(), projectID, destination, "", nil, headers); err != nil {
				return err
			}
			a.Output.Text(successText)
			return nil
		},
	}
	cmd.Flags().Bool("overwrite", false, "Overwrite the destination if it already exists (default: fail with 412 if exists)")
	return cmd
}

func renderFilesList(w *output.Writer, result []api.File) {
	if w.IsStructured() {
		w.Structured(result)
		return
	}
	if len(result) == 0 {
		w.Text("No files found.")
		return
	}
	if w.Format == output.FormatText && w.IsQuiet() {
		for _, f := range result {
			w.Text("%s", f.Path)
		}
		return
	}
	rows := make([][]string, len(result))
	for i, f := range result {
		name := f.Name
		if f.IsDir {
			name += "/"
		}
		rows[i] = []string{
			name,
			fmt.Sprintf("%d", f.Size),
			f.ModifiedAt.Format(time.RFC3339),
		}
	}
	w.Table([]string{"NAME", "SIZE", "MODIFIED"}, rows)
}
