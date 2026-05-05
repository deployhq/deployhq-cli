package commands

import (
	"fmt"

	"github.com/deployhq/deployhq-cli/internal/output"
	"github.com/spf13/cobra"
)

func newZonesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "zones",
		Short: "List available deployment zones",
		Long: `Geographic zones where DeployHQ runs its build infrastructure (US, EU, Asia-Pacific, etc.). Choosing a zone close to your servers reduces deploy latency and stays within data-residency boundaries.

Pin a zone per project at creation: "dhq projects create --zone <id>", or change later with "dhq projects update".`,
	}
	cmd.AddCommand(
		&cobra.Command{
			Use: "list", Short: "List zones",
			RunE: func(cmd *cobra.Command, args []string) error {
				client, err := cliCtx.Client()
				if err != nil {
					return err
				}
				zones, err := client.ListZones(cliCtx.Background(), nil)
				if err != nil {
					return err
				}
				env := cliCtx.Envelope
				if env.JSONMode || !env.IsTTY {
					return env.WriteJSON(output.NewResponse(zones, fmt.Sprintf("%d zones", len(zones))))
				}
				rows := make([][]string, len(zones))
				for i, z := range zones {
					rows[i] = []string{z.Identifier, z.Description}
				}
				env.WriteTable([]string{"Identifier", "Description"}, rows)
				env.Status("\nTip: dhq projects create --name myapp --zone <identifier>")
				return nil
			},
		},
	)
	return cmd
}
