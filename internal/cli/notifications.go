package cli

import (
	"time"

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
			result, err := a.API.ListNotifications(cmd.Context())
			if err != nil {
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
			if err := a.API.DeleteNotification(cmd.Context(), args[0]); err != nil {
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
		rows[i] = []string{n.Id, n.CreatedAt.Format(time.RFC3339), n.Priority, n.Title}
	}
	w.Table([]string{"ID", "CREATED", "PRIORITY", "TITLE"}, rows)
}
