package commands

import (
	"fmt"

	"github.com/deployhq/deployhq-cli/internal/output"
	"github.com/spf13/cobra"
)

func newScheduledDeploysCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "scheduled-deploys",
		Short: "Manage scheduled deployments",
	}
	cmd.AddCommand(
		&cobra.Command{
			Use: "list", Short: "List scheduled deployments",
			RunE: func(cmd *cobra.Command, args []string) error {
				projectID, err := cliCtx.RequireProject()
				if err != nil {
					return err
				}
				client, err := cliCtx.Client()
				if err != nil {
					return err
				}
				result, err := client.ListScheduledDeployments(cliCtx.Background(), projectID)
				if err != nil {
					return err
				}
				env := cliCtx.Envelope
				if env.JSONMode || !env.IsTTY {
					return env.WriteJSON(output.NewResponse(result, fmt.Sprintf("%d scheduled deployments", len(result))))
				}
				rows := make([][]string, len(result))
				for i, s := range result {
					rows[i] = []string{s.Identifier, s.Server, s.Frequency, s.NextDeploymentAt}
				}
				env.WriteTable([]string{"Identifier", "Server", "Frequency", "Next At"}, rows)
				return nil
			},
		},
		&cobra.Command{
			Use: "show <id>", Short: "Show a scheduled deployment", Args: cobra.ExactArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				projectID, err := cliCtx.RequireProject()
				if err != nil {
					return err
				}
				client, err := cliCtx.Client()
				if err != nil {
					return err
				}
				s, err := client.GetScheduledDeployment(cliCtx.Background(), projectID, args[0])
				if err != nil {
					return err
				}
				return cliCtx.Envelope.WriteJSON(output.NewResponse(s, fmt.Sprintf("Scheduled: %s (%s)", s.Identifier, s.Frequency)))
			},
		},
		&cobra.Command{
			Use: "delete <id>", Short: "Delete a scheduled deployment", Args: cobra.ExactArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				projectID, err := cliCtx.RequireProject()
				if err != nil {
					return err
				}
				client, err := cliCtx.Client()
				if err != nil {
					return err
				}
				if err := client.DeleteScheduledDeployment(cliCtx.Background(), projectID, args[0]); err != nil {
					return err
				}
				cliCtx.Envelope.Status("Deleted scheduled deployment: %s", args[0])
				return nil
			},
		},
	)
	return cmd
}
