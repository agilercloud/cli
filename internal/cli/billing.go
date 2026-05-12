package cli

import (
	"fmt"
	"io"
	"os"

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
		Short: "Show current balance, plan, and payment posture",
		RunE: func(cmd *cobra.Command, _ []string) error {
			var result api.Billing
			if err := a.API.DoJSON(cmd.Context(), "GET", "/v1/users/me/billing", nil, &result); err != nil {
				return err
			}
			return renderBillingStatus(a.Output, result)
		},
	})

	cmd.AddCommand(&cobra.Command{
		Use:   "transactions",
		Short: "List recent account transactions",
		RunE: func(cmd *cobra.Command, _ []string) error {
			var result []api.BillingTransaction
			if err := a.API.DoJSON(cmd.Context(), "GET", "/v1/users/me/billing/transactions", nil, &result); err != nil {
				return err
			}
			renderBillingTransactions(a.Output, result)
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
			path := fmt.Sprintf("/v1/users/me/billing/statements/%s", args[0])
			resp, err := a.API.Do(cmd.Context(), "GET", path, nil)
			if err != nil {
				return err
			}
			defer func() { _ = resp.Body.Close() }()

			var dest io.Writer
			var toClose io.Closer
			if outputPath == "" || outputPath == "-" {
				dest = a.Out
			} else {
				f, err := os.Create(outputPath)
				if err != nil {
					return fmt.Errorf("create output file: %w", err)
				}
				dest = f
				toClose = f
			}

			n, err := io.Copy(dest, resp.Body)
			if toClose != nil {
				_ = toClose.Close()
			}
			if err != nil {
				return fmt.Errorf("write file: %w", err)
			}
			if outputPath != "" && outputPath != "-" {
				a.Output.Stderr("Downloaded %d bytes to %s", n, outputPath)
			}
			return nil
		},
	}
	stmt.Flags().StringP("output", "o", "", "Output file path (default: stdout)")
	cmd.AddCommand(stmt)

	return cmd
}

func renderBillingStatus(w *output.Writer, b api.Billing) error {
	if w.IsTabular() {
		return tabularUnsupportedErr(w)
	}
	if w.IsStructured() {
		w.Structured(b)
		return nil
	}
	w.Text("Balance:  %.2f %s", b.Balance, b.Currency)
	if b.Plan != "" {
		w.Text("Plan:     %s", b.Plan)
	}
	if b.PaymentMethod != "" {
		w.Text("Method:   %s", b.PaymentMethod)
	}
	w.Text("Auto-pay: %s", boolYesNo(b.AutoPay))
	if b.UpdateRequired {
		w.Text("Update required: yes — please review billing settings in the web app.")
	}
	return nil
}

func renderBillingTransactions(w *output.Writer, txs []api.BillingTransaction) {
	if w.IsStructured() {
		w.Structured(txs)
		return
	}
	if len(txs) == 0 {
		w.Text("No transactions.")
		return
	}
	rows := make([][]string, len(txs))
	for i, t := range txs {
		rows[i] = []string{t.CreatedAt, t.Kind, t.Description, fmt.Sprintf("%.2f %s", t.Amount, t.Currency)}
	}
	w.Table([]string{"DATE", "KIND", "DESCRIPTION", "AMOUNT"}, rows)
}

func boolYesNo(b bool) string {
	if b {
		return "yes"
	}
	return "no"
}
