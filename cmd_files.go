package main

import (
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

	"github.com/spf13/cobra"
)

var filesCmd = &cobra.Command{
	Use:   "files",
	Short: "Manage project files",
}

func init() {
	filesCmd.AddCommand(filesListCmd)
	filesCmd.AddCommand(filesGetCmd)
	filesCmd.AddCommand(filesUploadCmd)
	filesCmd.AddCommand(filesDeleteCmd)
	filesCmd.AddCommand(filesMoveCmd)
}

func encodeFilePath(p string) string {
	p = strings.TrimPrefix(p, "/")
	segments := strings.Split(p, "/")
	for i, s := range segments {
		segments[i] = url.PathEscape(s)
	}
	return strings.Join(segments, "/")
}

// fileEntry is a typed representation of a remote file from the list endpoint.
type fileEntry struct {
	Name       string `json:"name"`
	Path       string `json:"path"`
	IsDir      bool   `json:"is_dir"`
	Size       int64  `json:"size"`
	ModifiedAt string `json:"modified_at"`
}

type syncStats struct {
	transferred int
	skipped     int
	errors      int
}

func (s *syncStats) print() {
	fmt.Fprintf(os.Stderr, "%d transferred, %d skipped, %d errors\n", s.transferred, s.skipped, s.errors)
}

// listRemoteDir lists the contents of a remote directory, returning typed entries.
func listRemoteDir(projectID, remotePath string) ([]fileEntry, error) {
	p := "/v1/projects/" + projectID + "/files"
	if remotePath != "" {
		p += "/" + encodeFilePath(remotePath)
	}
	var result []fileEntry
	if err := apiClient.DoJSON("GET", p, nil, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// shouldSkip returns true if the local file matches the remote size and mtime.
func shouldSkip(localPath string, remoteSize int64, remoteModifiedAt string) bool {
	info, err := os.Stat(localPath)
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

// remoteParentDir returns the parent directory of a remote path.
func remoteParentDir(remotePath string) string {
	remotePath = strings.TrimPrefix(remotePath, "/")
	dir := path.Dir(remotePath)
	if dir == "." {
		return ""
	}
	return dir
}

// --- Upload ---

func uploadSingleFile(projectID, remotePath, localPath string) error {
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
	resp, err := apiClient.DoRaw("PUT", p, "application/octet-stream", headers, f)
	if err != nil {
		return err
	}
	resp.Body.Close()
	return nil
}

func uploadDir(projectID, remoteBase, localDir string, force bool, stats *syncStats) error {
	entries, err := os.ReadDir(localDir)
	if err != nil {
		return fmt.Errorf("read directory: %w", err)
	}

	// Build remote lookup for skip checks.
	var remoteMap map[string]fileEntry
	if !force {
		remoteEntries, err := listRemoteDir(projectID, remoteBase)
		if err == nil {
			remoteMap = make(map[string]fileEntry, len(remoteEntries))
			for _, e := range remoteEntries {
				remoteMap[e.Name] = e
			}
		}
		// If listing fails (dir doesn't exist yet), remoteMap stays nil — all files are new.
	}

	for _, entry := range entries {
		// Skip symlinks to avoid cycles.
		if entry.Type()&os.ModeSymlink != 0 {
			continue
		}

		localPath := filepath.Join(localDir, entry.Name())
		remotePath := path.Join(remoteBase, entry.Name())

		if entry.IsDir() {
			if err := uploadDir(projectID, remotePath, localPath, force, stats); err != nil {
				return err
			}
			continue
		}

		// Check skip.
		if !force && remoteMap != nil {
			if re, ok := remoteMap[entry.Name()]; ok && !re.IsDir {
				if shouldSkip(localPath, re.Size, re.ModifiedAt) {
					fmt.Fprintf(os.Stderr, "skip %s\n", remotePath)
					stats.skipped++
					continue
				}
			}
		}

		if err := uploadSingleFile(projectID, remotePath, localPath); err != nil {
			fmt.Fprintf(os.Stderr, "error %s: %v\n", remotePath, err)
			stats.errors++
			continue
		}
		fmt.Fprintf(os.Stderr, "upload %s\n", remotePath)
		stats.transferred++
	}
	return nil
}

var filesUploadCmd = &cobra.Command{
	Use:   "upload <project> <remote-path> <local-path>",
	Short: "Upload a file or directory to a project",
	Args:  cobra.ExactArgs(3),
	RunE: func(cmd *cobra.Command, args []string) error {
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
			if err := uploadDir(projectID, remotePath, localPath, force, stats); err != nil {
				return err
			}
			stats.print()
			return nil
		}

		// Single file upload with skip check.
		if !force {
			parentDir := remoteParentDir(remotePath)
			remoteEntries, err := listRemoteDir(projectID, parentDir)
			if err == nil {
				baseName := path.Base(remotePath)
				for _, re := range remoteEntries {
					if re.Name == baseName && !re.IsDir {
						if shouldSkip(localPath, re.Size, re.ModifiedAt) {
							fmt.Fprintln(os.Stderr, "skip (unchanged)")
							return nil
						}
						break
					}
				}
			}
		}

		if err := uploadSingleFile(projectID, remotePath, localPath); err != nil {
			return err
		}
		fmt.Println("File uploaded.")
		return nil
	},
}

// --- Download ---

func downloadSingleFile(projectID, remotePath, localPath string) error {
	p := "/v1/projects/" + projectID + "/files/" + encodeFilePath(remotePath)
	resp, err := apiClient.Do("GET", p, nil)
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

func downloadDir(projectID, remoteBase, localDir string, force bool, stats *syncStats) error {
	entries, err := listRemoteDir(projectID, remoteBase)
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
			if err := downloadDir(projectID, remotePath, localPath, force, stats); err != nil {
				return err
			}
			continue
		}

		// Check skip.
		if !force && shouldSkip(localPath, entry.Size, entry.ModifiedAt) {
			fmt.Fprintf(os.Stderr, "skip %s\n", remotePath)
			stats.skipped++
			continue
		}

		if err := downloadSingleFile(projectID, remotePath, localPath); err != nil {
			fmt.Fprintf(os.Stderr, "error %s: %v\n", remotePath, err)
			stats.errors++
			continue
		}
		fmt.Fprintf(os.Stderr, "download %s\n", remotePath)
		stats.transferred++
	}
	return nil
}

var filesGetCmd = &cobra.Command{
	Use:   "get <project> <path>",
	Short: "Download a file or directory from a project",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		projectID := args[0]
		remotePath := args[1]
		output, _ := cmd.Flags().GetString("output")
		force, _ := cmd.Flags().GetBool("force")

		// Try listing as a directory first.
		entries, listErr := listRemoteDir(projectID, remotePath)
		isDir := listErr == nil && entries != nil

		if isDir {
			if output == "" || output == "-" {
				return fmt.Errorf("cannot download directory to stdout; use -o to specify output directory")
			}
			stats := &syncStats{}
			if err := downloadDir(projectID, remotePath, output, force, stats); err != nil {
				return err
			}
			stats.print()
			return nil
		}

		// Single file download.
		if output == "" || output == "-" {
			// Stream to stdout, no skip check.
			p := "/v1/projects/" + projectID + "/files/" + encodeFilePath(remotePath)
			resp, err := apiClient.Do("GET", p, nil)
			if err != nil {
				return err
			}
			defer resp.Body.Close()
			_, err = io.Copy(os.Stdout, resp.Body)
			return err
		}

		// Single file with skip check via parent directory listing.
		if !force {
			parentDir := remoteParentDir(remotePath)
			remoteEntries, err := listRemoteDir(projectID, parentDir)
			if err == nil {
				baseName := path.Base(remotePath)
				for _, re := range remoteEntries {
					if re.Name == baseName && !re.IsDir {
						if shouldSkip(output, re.Size, re.ModifiedAt) {
							fmt.Fprintln(os.Stderr, "skip (unchanged)")
							return nil
						}
						break
					}
				}
			}
		}

		if err := downloadSingleFile(projectID, remotePath, output); err != nil {
			return err
		}
		fmt.Fprintf(os.Stderr, "Downloaded to %s\n", output)
		return nil
	},
}

func init() {
	filesGetCmd.Flags().StringP("output", "o", "", "Output file or directory path (default: stdout)")
	filesGetCmd.Flags().BoolP("force", "f", false, "Force transfer even if file is unchanged")
	filesUploadCmd.Flags().BoolP("force", "f", false, "Force transfer even if file is unchanged")
}

// --- List ---

var filesListCmd = &cobra.Command{
	Use:   "list <project> [path]",
	Short: "List files in a project directory",
	Args:  cobra.RangeArgs(1, 2),
	RunE: func(cmd *cobra.Command, args []string) error {
		p := "/v1/projects/" + args[0] + "/files"
		if len(args) > 1 {
			p += "/" + encodeFilePath(args[1])
		}

		var result []map[string]any
		if err := apiClient.DoJSON("GET", p, nil, &result); err != nil {
			return err
		}

		if outputJSON {
			printJSON(result)
			return nil
		}

		if len(result) == 0 {
			fmt.Println("No files found.")
			return nil
		}

		if outputQuiet {
			for _, f := range result {
				fmt.Println(f["path"])
			}
			return nil
		}

		w := newTabWriter()
		fmt.Fprintln(w, "NAME\tSIZE\tMODIFIED")
		for _, f := range result {
			name := f["name"]
			if isDir, _ := f["is_dir"].(bool); isDir {
				name = fmt.Sprintf("%v/", name)
			}
			fmt.Fprintf(w, "%v\t%v\t%v\n", name, f["size"], f["modified_at"])
		}
		w.Flush()
		return nil
	},
}

// --- Delete ---

var filesDeleteCmd = &cobra.Command{
	Use:   "delete <project> <path>",
	Short: "Delete a file from a project",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		p := "/v1/projects/" + args[0] + "/files/" + encodeFilePath(args[1])
		if err := apiClient.DoJSON("DELETE", p, nil, nil); err != nil {
			return err
		}

		fmt.Println("File deleted.")
		return nil
	},
}

// --- Move ---

var filesMoveCmd = &cobra.Command{
	Use:   "move <project> <source> <destination>",
	Short: "Move or rename a file",
	Args:  cobra.ExactArgs(3),
	RunE: func(cmd *cobra.Command, args []string) error {
		overwrite, _ := cmd.Flags().GetBool("overwrite")

		body := fmt.Sprintf(`{"source":%q,"destination":%q,"overwrite":%t}`, args[1], args[2], overwrite)
		p := "/v1/projects/" + args[0] + "/files"
		if err := apiClient.DoJSON("PATCH", p, strings.NewReader(body), nil); err != nil {
			return err
		}

		fmt.Println("File moved.")
		return nil
	},
}

func init() {
	filesMoveCmd.Flags().Bool("overwrite", false, "Overwrite destination if it exists")
}
