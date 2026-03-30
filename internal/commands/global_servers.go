package commands

import (
	"fmt"

	"github.com/deployhq/deployhq-cli/internal/output"
	"github.com/spf13/cobra"
)

func newGlobalServersCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "global-servers",
		Short: "Manage global servers",
	}
	cmd.AddCommand(
		&cobra.Command{
			Use: "list", Short: "List global servers",
			RunE: func(cmd *cobra.Command, args []string) error {
				client, err := cliCtx.Client()
				if err != nil {
					return err
				}
				servers, err := client.ListGlobalServers(cliCtx.Background())
				if err != nil {
					return err
				}
				env := cliCtx.Envelope
				if env.JSONMode || !env.IsTTY {
					return env.WriteJSON(output.NewResponse(servers, fmt.Sprintf("%d global servers", len(servers))))
				}
				rows := make([][]string, len(servers))
				for i, s := range servers {
					rows[i] = []string{s.Name, s.Identifier, s.ProtocolType}
				}
				env.WriteTable([]string{"Name", "Identifier", "Protocol"}, rows)
				return nil
			},
		},
		&cobra.Command{
			Use: "show <id>", Short: "Show a global server", Args: cobra.ExactArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				client, err := cliCtx.Client()
				if err != nil {
					return err
				}
				s, err := client.GetGlobalServer(cliCtx.Background(), args[0])
				if err != nil {
					return err
				}
				return cliCtx.Envelope.WriteJSON(output.NewResponse(s, s.Name))
			},
		},
		&cobra.Command{
			Use: "delete <id>", Short: "Delete a global server", Args: cobra.ExactArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				client, err := cliCtx.Client()
				if err != nil {
					return err
				}
				if err := client.DeleteGlobalServer(cliCtx.Background(), args[0]); err != nil {
					return err
				}
				cliCtx.Envelope.Status("Deleted global server: %s", args[0])
				return nil
			},
		},
		newGlobalServersCopyCmd(),
	)
	return cmd
}

func newGlobalServersCopyCmd() *cobra.Command {
	var projectID string
	cmd := &cobra.Command{
		Use: "copy-to-project <server-id>", Short: "Copy a global server to a project", Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if projectID == "" {
				return &output.UserError{Message: "--project is required", Hint: "Specify which project to copy the server to"}
			}
			client, err := cliCtx.Client()
			if err != nil {
				return err
			}
			if err := client.CopyGlobalServerToProject(cliCtx.Background(), args[0], projectID); err != nil {
				return err
			}
			cliCtx.Envelope.Status("Copied global server %s to project %s", args[0], projectID)
			return nil
		},
	}
	cmd.Flags().StringVar(&projectID, "to-project", "", "Target project (required)")
	return cmd
}

func newIntegrationsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "integrations",
		Short: "Manage integrations (webhooks/notifications)",
	}
	cmd.AddCommand(
		&cobra.Command{
			Use: "list", Short: "List integrations",
			RunE: func(cmd *cobra.Command, args []string) error {
				projectID, err := cliCtx.RequireProject()
				if err != nil {
					return err
				}
				client, err := cliCtx.Client()
				if err != nil {
					return err
				}
				integrations, err := client.ListIntegrations(cliCtx.Background(), projectID)
				if err != nil {
					return err
				}
				env := cliCtx.Envelope
				if env.JSONMode || !env.IsTTY {
					return env.WriteJSON(output.NewResponse(integrations, fmt.Sprintf("%d integrations", len(integrations))))
				}
				rows := make([][]string, len(integrations))
				for i, ig := range integrations {
					rows[i] = []string{ig.Name, ig.Identifier, ig.HookType}
				}
				env.WriteTable([]string{"Name", "Identifier", "Type"}, rows)
				return nil
			},
		},
		&cobra.Command{
			Use: "show <id>", Short: "Show integration details", Args: cobra.ExactArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				projectID, err := cliCtx.RequireProject()
				if err != nil {
					return err
				}
				client, err := cliCtx.Client()
				if err != nil {
					return err
				}
				ig, err := client.GetIntegration(cliCtx.Background(), projectID, args[0])
				if err != nil {
					return err
				}
				return cliCtx.Envelope.WriteJSON(output.NewResponse(ig, ig.Name))
			},
		},
		&cobra.Command{
			Use: "delete <id>", Short: "Delete an integration", Args: cobra.ExactArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				projectID, err := cliCtx.RequireProject()
				if err != nil {
					return err
				}
				client, err := cliCtx.Client()
				if err != nil {
					return err
				}
				if err := client.DeleteIntegration(cliCtx.Background(), projectID, args[0]); err != nil {
					return err
				}
				cliCtx.Envelope.Status("Deleted integration: %s", args[0])
				return nil
			},
		},
	)
	return cmd
}
