package cli

import (
	"fmt"

	"github.com/agilercloud/cli/internal/api"
	"github.com/agilercloud/cli/internal/app"
	"github.com/agilercloud/cli/internal/output"
	"github.com/spf13/cobra"
)

func newNotificationsCmd(a *app.App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "notifications",
		Short: "List and dismiss account notifications",
	}

	cmd.AddCommand(&cobra.Command{
		Use:   "list",
		Short: "List notifications",
		RunE: func(cmd *cobra.Command, _ []string) error {
			var result []api.Notification
			if err := a.API.DoJSON(cmd.Context(), "GET", "/v1/users/me/notifications", nil, &result); err != nil {
				return err
			}
			renderNotificationsList(a.Output, result)
			return nil
		},
	})

	cmd.AddCommand(&cobra.Command{
		Use:   "delete <notification-id>",
		Short: "Delete a notification",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			path := fmt.Sprintf("/v1/users/me/notifications/%s", args[0])
			if err := a.API.DoJSON(cmd.Context(), "DELETE", path, nil, nil); err != nil {
				return err
			}
			a.Output.Text("Notification deleted.")
			return nil
		},
	})

	return cmd
}

func renderNotificationsList(w *output.Writer, ns []api.Notification) {
	if w.IsStructured() {
		w.Structured(ns)
		return
	}
	if len(ns) == 0 {
		w.Text("No notifications.")
		return
	}
	rows := make([][]string, len(ns))
	for i, n := range ns {
		read := "unread"
		if n.ReadAt != "" {
			read = "read"
		}
		rows[i] = []string{n.ID, n.CreatedAt, read, n.Subject}
	}
	w.Table([]string{"ID", "CREATED", "STATUS", "SUBJECT"}, rows)
}
