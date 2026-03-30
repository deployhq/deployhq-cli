package commands

import (
	"fmt"

	"github.com/deployhq/deployhq-cli/internal/output"
	"github.com/spf13/cobra"
)

func newAutoDeploysCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "auto-deploys",
		Short: "Manage auto deployments",
	}
	cmd.AddCommand(
		&cobra.Command{
			Use: "list", Short: "List auto deployment configuration",
			RunE: func(cmd *cobra.Command, args []string) error {
				projectID, err := cliCtx.RequireProject()
				if err != nil {
					return err
				}
				client, err := cliCtx.Client()
				if err != nil {
					return err
				}
				result, err := client.ListAutoDeployments(cliCtx.Background(), projectID)
				if err != nil {
					return err
				}
				env := cliCtx.Envelope
				if env.JSONMode || !env.IsTTY {
					return env.WriteJSON(output.NewResponse(result, fmt.Sprintf("Auto deploy URL: %s", result.WebhookURL)))
				}
				env.Status("Webhook URL: %s", result.WebhookURL)
				if len(result.Deployables) > 0 {
					rows := make([][]string, len(result.Deployables))
					for i, d := range result.Deployables {
						auto := "no"
						if d.AutoDeploy {
							auto = "yes"
						}
						rows[i] = []string{d.Name, d.Identifier, d.Type, d.PreferredBranch, auto}
					}
					env.WriteTable([]string{"Name", "Identifier", "Type", "Branch", "Auto"}, rows)
				}
				return nil
			},
		},
	)
	return cmd
}
