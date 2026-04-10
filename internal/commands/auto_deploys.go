package commands

import (
	"fmt"

	"github.com/deployhq/deployhq-cli/internal/output"
	"github.com/deployhq/deployhq-cli/pkg/sdk"
	"github.com/spf13/cobra"
)

func newAutoDeploysCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "auto-deploys",
		Short: "Manage auto deployments",
	}
	cmd.AddCommand(
		newAutoDeploysEnableCmd(),
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
				result, err := client.ListAutoDeployments(cliCtx.Background(), projectID, nil)
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

func newAutoDeploysEnableCmd() *cobra.Command {
	var serverID string
	var disable bool
	cmd := &cobra.Command{
		Use: "enable", Short: "Enable or disable auto deploy for a server",
		RunE: func(cmd *cobra.Command, args []string) error {
			if serverID == "" {
				return &output.UserError{Message: "--server is required"}
			}
			projectID, err := cliCtx.RequireProject()
			if err != nil {
				return err
			}
			client, err := cliCtx.Client()
			if err != nil {
				return err
			}
			result, err := client.CreateAutoDeployment(cliCtx.Background(), projectID, sdk.AutoDeployCreateRequest{
				Deployables: []sdk.DeployableToggle{{Identifier: serverID, AutoDeploy: !disable}},
			})
			if err != nil {
				return err
			}
			env := cliCtx.Envelope
			if env.JSONMode || !env.IsTTY {
				action := "Enabled"
				if disable {
					action = "Disabled"
				}
				return env.WriteJSON(output.NewResponse(result, fmt.Sprintf("%s auto deploy for %s", action, serverID)))
			}
			if disable {
				env.Status("Disabled auto deploy for: %s", serverID)
			} else {
				env.Status("Enabled auto deploy for: %s", serverID)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&serverID, "server", "", "Server or group identifier (required)")
	cmd.Flags().BoolVar(&disable, "disable", false, "Disable instead of enable")
	return cmd
}
