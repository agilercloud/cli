package cli

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strconv"
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

	return cmd
}

func encodeFilePath(p string) string {
	p = strings.TrimPrefix(p, "/")
	segments := strings.Split(p, "/")
	for i, s := range segments {
		segments[i] = url.PathEscape(s)
	}
	return strings.Join(segments, "/")
}

type syncStats struct {
	transferred int
	skipped     int
	errors      int
}

func (s *syncStats) print(w *output.Writer) {
	w.Stderr("%d transferred, %d skipped, %d errors", s.transferred, s.skipped, s.errors)
}

func listRemoteDir(ctx context.Context, client app.APIClient, projectID, remotePath string) ([]api.File, error) {
	p := "/v1/projects/" + projectID + "/files"
	if remotePath != "" {
		p += "/" + encodeFilePath(remotePath)
	}
	var result []api.File
	if err := client.DoJSON(ctx, "GET", p, nil, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// shouldSkip returns true if the local file matches the remote size and mtime.
func shouldSkip(fs fsx.FS, localPath string, remoteSize int64, remoteModifiedAt string) bool {
	info, err := fs.Stat(localPath)
	if err != nil {
		return false
	}
	if info.Size() != remoteSize {
		return false
	}
	t, err := time.Parse(time.RFC3339, remoteModifiedAt)
	if err != nil {
		t, err = time.Parse(time.RFC3339Nano, remoteModifiedAt)
		if err != nil {
			return false
		}
	}
	return info.ModTime().Unix() == t.Unix()
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

func uploadSingleFile(ctx context.Context, client app.APIClient, projectID, remotePath, localPath string) error {
	f, err := os.Open(localPath)
	if err != nil {
		return fmt.Errorf("open file: %w", err)
	}
	defer f.Close()

	headers := map[string]string{}
	if fi, err := f.Stat(); err == nil {
		headers["X-Modified-At"] = strconv.FormatInt(fi.ModTime().Unix(), 10)
	}

	p := "/v1/projects/" + projectID + "/files/" + encodeFilePath(remotePath)
	resp, err := client.DoRaw(ctx, "PUT", p, "application/octet-stream", headers, f)
	if err != nil {
		return err
	}
	resp.Body.Close()
	return nil
}

func uploadDir(ctx context.Context, a *app.App, projectID, remoteBase, localDir string, force bool, stats *syncStats) error {
	entries, err := os.ReadDir(localDir)
	if err != nil {
		return fmt.Errorf("read directory: %w", err)
	}

	var remoteMap map[string]api.File
	if !force {
		remoteEntries, err := listRemoteDir(ctx, a.API, projectID, remoteBase)
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
			if err := uploadDir(ctx, a, projectID, remotePath, localPath, force, stats); err != nil {
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

		if err := uploadSingleFile(ctx, a.API, projectID, remotePath, localPath); err != nil {
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

			fi, err := os.Stat(localPath)
			if err != nil {
				return fmt.Errorf("stat local path: %w", err)
			}

			if fi.IsDir() {
				stats := &syncStats{}
				if err := uploadDir(ctx, a, projectID, remotePath, localPath, force, stats); err != nil {
					return err
				}
				stats.print(a.Output)
				return nil
			}

			if !force {
				parentDir := remoteParentDir(remotePath)
				remoteEntries, err := listRemoteDir(ctx, a.API, projectID, parentDir)
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

			if err := uploadSingleFile(ctx, a.API, projectID, remotePath, localPath); err != nil {
				return err
			}
			a.Output.Text("File uploaded.")
			return nil
		},
	}
	cmd.Flags().BoolP("force", "f", false, "Force transfer even if file is unchanged")
	return cmd
}

// --- Download ---

func downloadSingleFile(ctx context.Context, client app.APIClient, projectID, remotePath, localPath string) error {
	p := "/v1/projects/" + projectID + "/files/" + encodeFilePath(remotePath)
	resp, err := client.Do(ctx, "GET", p, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if err := os.MkdirAll(filepath.Dir(localPath), 0o755); err != nil {
		return fmt.Errorf("create parent directory: %w", err)
	}

	dest, err := os.Create(localPath)
	if err != nil {
		return fmt.Errorf("create output file: %w", err)
	}
	defer dest.Close()

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
	entries, err := listRemoteDir(ctx, a.API, projectID, remoteBase)
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

			entries, listErr := listRemoteDir(ctx, a.API, projectID, remotePath)
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
				p := "/v1/projects/" + projectID + "/files/" + encodeFilePath(remotePath)
				resp, err := a.API.Do(ctx, "GET", p, nil)
				if err != nil {
					return err
				}
				defer resp.Body.Close()
				_, err = io.Copy(a.Out, resp.Body)
				return err
			}

			if !force {
				parentDir := remoteParentDir(remotePath)
				remoteEntries, err := listRemoteDir(ctx, a.API, projectID, parentDir)
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
			p := "/v1/projects/" + args[0] + "/files"
			if len(args) > 1 {
				p += "/" + encodeFilePath(args[1])
			}
			var result []api.File
			if err := a.API.DoJSON(cmd.Context(), "GET", p, nil, &result); err != nil {
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
			p := "/v1/projects/" + args[0] + "/files/" + encodeFilePath(args[1])
			if err := a.API.DoJSON(cmd.Context(), "DELETE", p, nil, nil); err != nil {
				return err
			}
			a.Output.Text("File deleted.")
			return nil
		},
	}
}

func newFilesMoveCmd(a *app.App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "move <project> <source> <destination>",
		Short: "Move or rename a file",
		Args:  cobra.ExactArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			overwrite, _ := cmd.Flags().GetBool("overwrite")

			body := fmt.Sprintf(`{"source":%q,"destination":%q,"overwrite":%t}`, args[1], args[2], overwrite)
			p := "/v1/projects/" + args[0] + "/files"
			if err := a.API.DoJSON(cmd.Context(), "PATCH", p, strings.NewReader(body), nil); err != nil {
				return err
			}
			a.Output.Text("File moved.")
			return nil
		},
	}
	cmd.Flags().Bool("overwrite", false, "Overwrite destination if it exists")
	return cmd
}

func renderFilesList(w *output.Writer, result []api.File) {
	if w.IsJSON() {
		w.JSON(result)
		return
	}
	if len(result) == 0 {
		w.Text("No files found.")
		return
	}
	if w.IsQuiet() {
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
			f.ModifiedAt,
		}
	}
	w.Table([]string{"NAME", "SIZE", "MODIFIED"}, rows)
}
