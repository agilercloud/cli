package cli

import (
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/agilercloud/cli/internal/api"
	"github.com/agilercloud/cli/internal/app"
	"github.com/agilercloud/cli/internal/output"
	"github.com/spf13/cobra"
)

func newBillingCmd(a *app.App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "billing",
		Short: "View account billing state, transactions, and statements",
	}

	cmd.AddCommand(&cobra.Command{
		Use:   "status",
		Short: "Show card on file, monthly budget, and update posture",
		RunE: func(cmd *cobra.Command, _ []string) error {
			result, err := a.API.GetBilling(cmd.Context())
			if err != nil {
				return err
			}
			return renderBillingStatus(a.Output, *result)
		},
	})

	cmd.AddCommand(&cobra.Command{
		Use:   "transactions",
		Short: "List recent account transactions",
		RunE: func(cmd *cobra.Command, _ []string) error {
			months, err := a.API.ListBillingTransactions(cmd.Context())
			if err != nil {
				return err
			}
			renderBillingTransactions(a.Output, months)
			return nil
		},
	})

	stmt := &cobra.Command{
		Use:   "statement <period>",
		Short: "Download a billing statement PDF",
		Long:  "Downloads the billing statement PDF for the given period (YYYY-MM). Writes to stdout by default; use -o to write to a file.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			outputPath, _ := cmd.Flags().GetString("output")
			resp, err := a.API.GetBillingStatement(cmd.Context(), args[0])
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

			n, err := writeStreamAtomic(outputPath, resp.Body)
			if err != nil {
				return err
			}
			a.Output.Stderr("Downloaded %d bytes to %s", n, outputPath)
			return nil
		},
	}
	stmt.Flags().StringP("output", "o", "", "Output file path (default: stdout)")
	cmd.AddCommand(stmt)

	return cmd
}

// renderBillingStatus prints the fields the public spec actually
// returns: cardholder name, card brand/last4/expiry, monthly budget
// and budget-alert thresholds, and the forced-update flag.
func renderBillingStatus(w *output.Writer, b api.Billing) error {
	if w.IsTabular() {
		return tabularUnsupportedErr(w)
	}
	if w.IsStructured() {
		w.Structured(b)
		return nil
	}
	if b.Name != "" {
		w.Text("Name:           %s", b.Name)
	}
	if b.Brand != "" || b.Last4 != "" {
		w.Text("Card:           %s ending in %s (exp %02d/%d)", b.Brand, b.Last4, b.ExpMonth, b.ExpYear)
	}
	if b.MonthlyBudget > 0 {
		w.Text("Monthly budget: $%.2f", float64(b.MonthlyBudget)/100)
	}
	if len(b.BudgetAlerts) > 0 {
		alerts := make([]string, len(b.BudgetAlerts))
		for i, a := range b.BudgetAlerts {
			alerts[i] = fmt.Sprintf("%d%%", a)
		}
		w.Text("Budget alerts:  %s", strings.Join(alerts, ", "))
	}
	w.Text("Budget stop:    %s", boolYesNo(b.BudgetStop))
	if b.ForceUpdate != nil {
		w.Text("Update required: yes — please review billing settings in the web app (since %s).",
			b.ForceUpdate.Format(time.RFC3339))
	}
	return nil
}

// renderBillingTransactions flattens months into rows for tabular text/CSV/TSV
// output. Structured output preserves the grouped shape.
func renderBillingTransactions(w *output.Writer, months []api.BillingMonth) {
	if w.IsStructured() {
		w.Structured(months)
		return
	}
	if len(months) == 0 {
		w.Text("No transactions.")
		return
	}
	var rows [][]string
	for _, m := range months {
		for _, t := range m.Tx {
			rows = append(rows, []string{
				t.CreatedAt.Format(time.RFC3339),
				t.Description,
				fmt.Sprintf("$%.2f", float64(t.Amount)/100),
			})
		}
	}
	if len(rows) == 0 {
		w.Text("No transactions.")
		return
	}
	w.Table([]string{"DATE", "DESCRIPTION", "AMOUNT"}, rows)
}

func boolYesNo(b bool) string {
	if b {
		return "yes"
	}
	return "no"
}
