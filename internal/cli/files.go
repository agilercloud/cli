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
		Use:   "files",
		Short: "Manage project files",
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

func uploadSingleFile(ctx context.Context, client *api.Client, projectID, remotePath, localPath string, overwrite bool) error {
	f, err := os.Open(localPath)
	if err != nil {
		return fmt.Errorf("open file: %w", err)
	}
	defer func() { _ = f.Close() }()

	headers := map[string]string{}
	if fi, err := f.Stat(); err == nil {
		headers["Last-Modified"] = fi.ModTime().UTC().Format(http.TimeFormat)
	}
	if !overwrite {
		headers["If-None-Match"] = "*"
	}
	return client.PutProjectFile(ctx, projectID, remotePath, "application/octet-stream", f, headers)
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

		if err := uploadSingleFile(ctx, a.API, projectID, remotePath, localPath, overwrite); err != nil {
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
		Use:   "upload <project> <remote-path> <local-path>",
		Short: "Upload a file or directory to a project",
		Args:  cobra.ExactArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			projectID := args[0]
			remotePath := args[1]
			localPath := args[2]
			force, _ := cmd.Flags().GetBool("force")
			overwrite, _ := cmd.Flags().GetBool("overwrite")

			fi, err := os.Stat(localPath)
			if err != nil {
				return fmt.Errorf("stat local path: %w", err)
			}

			if fi.IsDir() {
				stats := &syncStats{}
				if err := uploadDir(ctx, a, projectID, remotePath, localPath, force, overwrite, stats); err != nil {
					return err
				}
				stats.print(a.Output)
				return nil
			}

			if !force {
				parentDir := remoteParentDir(remotePath)
				remoteEntries, err := a.API.ListProjectFiles(ctx, projectID, parentDir)
				if err == nil {
					baseName := path.Base(remotePath)
					for _, re := range remoteEntries {
						if re.Name == baseName && !re.IsDir {
							if shouldSkip(a.FS, localPath, re.Size, re.ModifiedAt) {
								a.Output.Stderr("skip (unchanged)")
								return nil
							}
							break
						}
					}
				}
			}

			if err := uploadSingleFile(ctx, a.API, projectID, remotePath, localPath, overwrite); err != nil {
				return err
			}
			a.Output.Text("File uploaded.")
			return nil
		},
	}
	cmd.Flags().BoolP("force", "f", false, "Force transfer even if file is unchanged")
	cmd.Flags().BoolP("overwrite", "o", false, "Overwrite the remote destination if it already exists (default: fail with 412 if exists)")
	return cmd
}

// --- Download ---

func downloadSingleFile(ctx context.Context, client *api.Client, projectID, remotePath, localPath string) error {
	resp, err := client.GetProjectFile(ctx, projectID, remotePath)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	if err := os.MkdirAll(filepath.Dir(localPath), 0o755); err != nil {
		return fmt.Errorf("create parent directory: %w", err)
	}

	dest, err := os.Create(localPath)
	if err != nil {
		return fmt.Errorf("create output file: %w", err)
	}
	defer func() { _ = dest.Close() }()

	if _, err := io.Copy(dest, resp.Body); err != nil {
		return fmt.Errorf("write file: %w", err)
	}

	if lastMod := resp.Header.Get("Last-Modified"); lastMod != "" {
		if t, err := http.ParseTime(lastMod); err == nil {
			_ = os.Chtimes(localPath, t, t)
		}
	}
	return nil
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

		if err := downloadSingleFile(ctx, a.API, projectID, remotePath, localPath); err != nil {
			a.Output.Stderr("error %s: %v", remotePath, err)
			stats.errors++
			continue
		}
		a.Output.Stderr("download %s", remotePath)
		stats.transferred++
	}
	return nil
}

func newFilesGetCmd(a *app.App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "get <project> <path>",
		Short: "Download a file or directory from a project",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			projectID := args[0]
			remotePath := args[1]
			outputPath, _ := cmd.Flags().GetString("output")
			force, _ := cmd.Flags().GetBool("force")

			entries, listErr := a.API.ListProjectFiles(ctx, projectID, remotePath)
			isDir := listErr == nil && entries != nil

			if isDir {
				if outputPath == "" || outputPath == "-" {
					return fmt.Errorf("cannot download directory to stdout; use -o to specify output directory")
				}
				stats := &syncStats{}
				if err := downloadDir(ctx, a, projectID, remotePath, outputPath, force, stats); err != nil {
					return err
				}
				stats.print(a.Output)
				return nil
			}

			if outputPath == "" || outputPath == "-" {
				resp, err := a.API.GetProjectFile(ctx, projectID, remotePath)
				if err != nil {
					return err
				}
				defer func() { _ = resp.Body.Close() }()
				_, err = io.Copy(a.Out, resp.Body)
				return err
			}

			if !force {
				parentDir := remoteParentDir(remotePath)
				remoteEntries, err := a.API.ListProjectFiles(ctx, projectID, parentDir)
				if err == nil {
					baseName := path.Base(remotePath)
					for _, re := range remoteEntries {
						if re.Name == baseName && !re.IsDir {
							if shouldSkip(a.FS, outputPath, re.Size, re.ModifiedAt) {
								a.Output.Stderr("skip (unchanged)")
								return nil
							}
							break
						}
					}
				}
			}

			if err := downloadSingleFile(ctx, a.API, projectID, remotePath, outputPath); err != nil {
				return err
			}
			a.Output.Stderr("Downloaded to %s", outputPath)
			return nil
		},
	}
	cmd.Flags().StringP("output", "o", "", "Output file or directory path (default: stdout)")
	cmd.Flags().BoolP("force", "f", false, "Force transfer even if file is unchanged")
	return cmd
}

// --- List ---

func newFilesListCmd(a *app.App) *cobra.Command {
	return &cobra.Command{
		Use:   "list <project> [path]",
		Short: "List files in a project directory",
		Args:  cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			var sub string
			if len(args) > 1 {
				sub = args[1]
			}
			result, err := a.API.ListProjectFiles(cmd.Context(), args[0], sub)
			if err != nil {
				return err
			}
			renderFilesList(a.Output, result)
			return nil
		},
	}
}

func newFilesDeleteCmd(a *app.App) *cobra.Command {
	return &cobra.Command{
		Use:   "delete <project> <path>",
		Short: "Delete a file from a project",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := a.API.DeleteProjectFile(cmd.Context(), args[0], args[1]); err != nil {
				return err
			}
			a.Output.Text("File deleted.")
			return nil
		},
	}
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
		Use:   name + " <project> <source> <destination>",
		Short: short,
		Args:  cobra.ExactArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			overwrite, _ := cmd.Flags().GetBool("overwrite")
			projectID := args[0]
			source := args[1]
			destination := args[2]

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
	cmd.Flags().BoolP("overwrite", "o", false, "Overwrite the destination if it already exists (default: fail with 412 if exists)")
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
