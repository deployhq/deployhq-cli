package commands

import (
	"fmt"
	"os"
	"strconv"

	"github.com/deployhq/deployhq-cli/internal/output"
	"github.com/spf13/cobra"
)

func newInvoicesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "invoices",
		Short: "List and download account invoices",
		Long: `Billing invoices on your account.

Requires a billing-manager API key; other keys receive a 403. Invoices are keyed
by number. The single-invoice endpoint is PDF-only, so "download" writes the raw
PDF (there is no JSON view of an individual invoice).`,
	}
	cmd.AddCommand(newInvoicesListCmd(), newInvoicesDownloadCmd())
	return cmd
}

func newInvoicesListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List invoices",
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := cliCtx.Client()
			if err != nil {
				return err
			}
			invoices, err := client.ListInvoices(cliCtx.Background(), nil)
			if err != nil {
				return err
			}
			env := cliCtx.Envelope
			if env.WantsJSON() {
				return env.WriteJSON(output.NewResponse(invoices, fmt.Sprintf("%d invoices", len(invoices))))
			}
			rows := make([][]string, len(invoices))
			for i, inv := range invoices {
				paid := "no"
				if inv.Paid {
					paid = "yes"
				}
				rows[i] = []string{
					strconv.Itoa(inv.Number),
					inv.CreatedAt,
					inv.Total,
					inv.Currency,
					paid,
				}
			}
			env.WriteTable([]string{"Number", "Date", "Total", "Currency", "Paid"}, rows)
			env.Status("\nTip: dhq invoices download <number> --output invoice.pdf")
			return nil
		},
	}
}

func newInvoicesDownloadCmd() *cobra.Command {
	var outputPath string
	cmd := &cobra.Command{
		Use:   "download <number>",
		Short: "Download an invoice PDF",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			number, err := strconv.Atoi(args[0])
			if err != nil {
				return fmt.Errorf("invalid invoice number %q: must be an integer", args[0])
			}
			client, err := cliCtx.Client()
			if err != nil {
				return err
			}
			data, err := client.DownloadInvoice(cliCtx.Background(), number)
			if err != nil {
				return err
			}
			env := cliCtx.Envelope

			// No --output and piped: stream the PDF to stdout so it can be
			// redirected. Otherwise write to a file (default filename).
			if outputPath == "" && !env.IsTTY {
				_, werr := env.Stdout.Write(data)
				return werr
			}
			if outputPath == "" {
				outputPath = fmt.Sprintf("invoice-%d.pdf", number)
			}
			if err := os.WriteFile(outputPath, data, 0o644); err != nil {
				return fmt.Errorf("write %s: %w", outputPath, err)
			}
			env.Status("Saved invoice %d to %s (%d bytes)", number, outputPath, len(data))
			return nil
		},
	}
	cmd.Flags().StringVarP(&outputPath, "output", "o", "", "Write the PDF to this file (default invoice-<number>.pdf)")
	return cmd
}
