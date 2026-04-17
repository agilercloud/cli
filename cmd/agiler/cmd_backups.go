package main

import (
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"
)

var backupsCmd = &cobra.Command{
	Use:   "backups",
	Short: "Manage project backups",
}

func init() {
	backupsCmd.AddCommand(backupsListCmd)
	backupsCmd.AddCommand(backupsCreateCmd)
	backupsCmd.AddCommand(backupsDeleteCmd)
	backupsCmd.AddCommand(backupsRestoreCmd)
	backupsCmd.AddCommand(backupsDownloadCmd)
}

var backupsListCmd = &cobra.Command{
	Use:   "list <project>",
	Short: "List project backups",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		var result map[string]any
		if err := apiClient.DoJSON("GET", fmt.Sprintf("/v1/projects/%s/backups", args[0]), nil, &result); err != nil {
			return err
		}

		if outputJSON {
			printJSON(result)
			return nil
		}

		backups, _ := result["data"].([]any)
		if len(backups) == 0 {
			fmt.Println("No backups found.")
			return nil
		}

		if outputQuiet {
			for _, b := range backups {
				if bm, ok := b.(map[string]any); ok {
					fmt.Println(bm["id"])
				}
			}
			return nil
		}

		w := newTabWriter()
		fmt.Fprintln(w, "ID\tSTATUS\tCREATED\tAUTO\tSIZE (MB)")
		for _, b := range backups {
			if bm, ok := b.(map[string]any); ok {
				fmt.Fprintf(w, "%v\t%v\t%v\t%v\t%v\n",
					bm["id"], bm["status"], bm["created_at"], bm["automatic"], bm["size"])
			}
		}
		w.Flush()

		fmt.Printf("\nBackup schedule: every %v hours, retain %v\n", result["frequency"], result["retention"])
		return nil
	},
}

var backupsCreateCmd = &cobra.Command{
	Use:   "create <project>",
	Short: "Create a manual backup",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := apiClient.DoJSONWithIdempotency("POST", fmt.Sprintf("/v1/projects/%s/backups", args[0]), nil, nil); err != nil {
			return err
		}

		fmt.Println("Backup created.")
		return nil
	},
}

var backupsDeleteCmd = &cobra.Command{
	Use:   "delete <project> <backup-id>",
	Short: "Delete a backup",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := apiClient.DoJSON("DELETE", fmt.Sprintf("/v1/projects/%s/backups/%s", args[0], args[1]), nil, nil); err != nil {
			return err
		}

		fmt.Println("Backup deleted.")
		return nil
	},
}

var backupsRestoreCmd = &cobra.Command{
	Use:   "restore <project> <backup-id>",
	Short: "Restore a backup",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := apiClient.DoJSONWithIdempotency("POST", fmt.Sprintf("/v1/projects/%s/backups/%s/restore", args[0], args[1]), nil, nil); err != nil {
			return err
		}

		fmt.Println("Backup restore initiated.")
		return nil
	},
}

var backupsDownloadCmd = &cobra.Command{
	Use:   "download <project> <backup-id>",
	Short: "Download a backup",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		dlType, _ := cmd.Flags().GetString("type")
		if dlType != "storage" && dlType != "database" {
			return fmt.Errorf("--type must be 'storage' or 'database'")
		}

		output, _ := cmd.Flags().GetString("output")

		path := fmt.Sprintf("/v1/projects/%s/backups/%s/%s", args[0], args[1], dlType)
		resp, err := apiClient.Do("GET", path, nil)
		if err != nil {
			return err
		}
		defer resp.Body.Close()

		var dest *os.File
		if output == "" || output == "-" {
			dest = os.Stdout
		} else {
			dest, err = os.Create(output)
			if err != nil {
				return fmt.Errorf("create output file: %w", err)
			}
			defer dest.Close()
		}

		n, err := io.Copy(dest, resp.Body)
		if err != nil {
			return fmt.Errorf("write file: %w", err)
		}

		if output != "" && output != "-" {
			fmt.Fprintf(os.Stderr, "Downloaded %d bytes to %s\n", n, output)
		}
		return nil
	},
}

func init() {
	backupsDownloadCmd.Flags().String("type", "", "Download type: 'storage' or 'database' (required)")
	backupsDownloadCmd.Flags().StringP("output", "o", "", "Output file path (default: stdout)")
	backupsDownloadCmd.MarkFlagRequired("type")
}
