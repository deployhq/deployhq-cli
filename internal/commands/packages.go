package commands

import (
	"fmt"
	"strconv"

	"github.com/deployhq/deployhq-cli/internal/output"
	"github.com/spf13/cobra"
)

func newPlansCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "plans",
		Aliases: []string{"pricing"},
		Short:   "List DeployHQ subscription plans and pricing",
		Long: `DeployHQ's subscription plans with pricing localised to your location.

Prices are quoted in the currency inferred from your IP, so they may differ by
region. This endpoint is public. Use --json to include each plan's feature list.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := cliCtx.PublicClient()
			if err != nil {
				return err
			}
			plans, err := client.ListPackages(cliCtx.Background())
			if err != nil {
				return err
			}
			env := cliCtx.Envelope
			if env.WantsJSON() {
				return env.WriteJSON(output.NewResponse(plans, fmt.Sprintf("%d plans", len(plans))))
			}
			if env.QuietMode {
				permalinks := make([]string, len(plans))
				for i, p := range plans {
					permalinks[i] = p.Permalink
				}
				env.WriteQuiet(permalinks)
				return nil
			}
			rows := make([][]string, len(plans))
			for i, p := range plans {
				rows[i] = []string{
					p.Permalink,
					p.Name,
					strconv.FormatFloat(p.Price, 'f', -1, 64),
					strconv.FormatFloat(p.PriceBilledAnnually, 'f', -1, 64),
					p.Currency,
				}
			}
			env.WriteTable([]string{"Permalink", "Name", "Price", "Annual", "Currency"}, rows)
			env.Status("\nTip: dhq plans --json for each plan's feature list")
			return nil
		},
	}
	return cmd
}
