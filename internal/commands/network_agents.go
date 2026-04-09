package commands

import (
	"fmt"

	"github.com/deployhq/deployhq-cli/internal/output"
	"github.com/deployhq/deployhq-cli/pkg/sdk"
	"github.com/spf13/cobra"
)

func newAgentsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "agents",
		Short: "Manage network agents",
	}
	cmd.AddCommand(
		&cobra.Command{
			Use: "list", Short: "List network agents",
			RunE: func(cmd *cobra.Command, args []string) error {
				client, err := cliCtx.Client()
				if err != nil {
					return err
				}
				agents, err := client.ListAgents(cliCtx.Background(), nil)
				if err != nil {
					return err
				}
				env := cliCtx.Envelope
				if env.JSONMode || !env.IsTTY {
					return env.WriteJSON(output.NewResponse(agents, fmt.Sprintf("%d agents", len(agents))))
				}
				rows := make([][]string, len(agents))
				for i, a := range agents {
					online := "offline"
					if a.Online {
						online = "online"
					}
					rows[i] = []string{a.Name, a.Identifier, output.ColorStatus(online)}
				}
				env.WriteTable([]string{"Name", "Identifier", "Status"}, rows)
				return nil
			},
		},
		newAgentsCreateCmd(),
		newAgentsUpdateCmd(),
		&cobra.Command{
			Use: "delete <id>", Short: "Delete an agent", Args: cobra.ExactArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				client, err := cliCtx.Client()
				if err != nil {
					return err
				}
				if err := client.DeleteAgent(cliCtx.Background(), args[0]); err != nil {
					return err
				}
				cliCtx.Envelope.Status("Deleted agent: %s", args[0])
				return nil
			},
		},
		&cobra.Command{
			Use: "revoke <id>", Short: "Revoke an agent", Args: cobra.ExactArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				client, err := cliCtx.Client()
				if err != nil {
					return err
				}
				if err := client.RevokeAgent(cliCtx.Background(), args[0]); err != nil {
					return err
				}
				cliCtx.Envelope.Status("Revoked agent: %s", args[0])
				return nil
			},
		},
	)
	return cmd
}

func newAgentsUpdateCmd() *cobra.Command {
	var name string
	cmd := &cobra.Command{
		Use: "update <id>", Short: "Update an agent", Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if name == "" {
				return &output.UserError{Message: "--name is required"}
			}
			client, err := cliCtx.Client()
			if err != nil {
				return err
			}
			a, err := client.UpdateAgent(cliCtx.Background(), args[0], name)
			if err != nil {
				return err
			}
			cliCtx.Envelope.Status("Updated agent: %s (%s)", a.Name, a.Identifier)
			return nil
		},
	}
	cmd.Flags().StringVar(&name, "name", "", "Agent name (required)")
	return cmd
}

func newAgentsCreateCmd() *cobra.Command {
	var claimCode string
	cmd := &cobra.Command{
		Use: "create", Short: "Register an agent by claim code",
		RunE: func(cmd *cobra.Command, args []string) error {
			if claimCode == "" {
				return &output.UserError{Message: "--claim-code is required"}
			}
			client, err := cliCtx.Client()
			if err != nil {
				return err
			}
			a, err := client.CreateAgent(cliCtx.Background(), sdk.AgentCreateRequest{ClaimCode: claimCode})
			if err != nil {
				return err
			}
			cliCtx.Envelope.Status("Registered agent: %s (%s)", a.Name, a.Identifier)
			return nil
		},
	}
	cmd.Flags().StringVar(&claimCode, "claim-code", "", "Agent claim code (required)")
	return cmd
}
