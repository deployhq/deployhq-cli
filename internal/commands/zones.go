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
	}
	cmd.AddCommand(
		&cobra.Command{
			Use: "list", Short: "List zones",
			RunE: func(cmd *cobra.Command, args []string) error {
				client, err := cliCtx.Client()
				if err != nil {
					return err
				}
				zones, err := client.ListZones(cliCtx.Background())
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
