package commands

import (
	"fmt"

	"github.com/deployhq/deployhq-cli/internal/output"
	"github.com/deployhq/deployhq-cli/pkg/sdk"
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
		newGlobalServersCreateCmd(),
		newGlobalServersUpdateCmd(),
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

func newGlobalServersCreateCmd() *cobra.Command {
	var name, protocol, serverPath, environment string
	cmd := &cobra.Command{
		Use: "create", Short: "Create a global server",
		RunE: func(cmd *cobra.Command, args []string) error {
			if name == "" || protocol == "" {
				return &output.UserError{Message: "Both --name and --protocol are required"}
			}
			client, err := cliCtx.Client()
			if err != nil {
				return err
			}
			s, err := client.CreateGlobalServer(cliCtx.Background(), sdk.ServerCreateRequest{
				Name: name, ProtocolType: protocol, ServerPath: serverPath, Environment: environment,
			})
			if err != nil {
				return err
			}
			env := cliCtx.Envelope
			if env.JSONMode || !env.IsTTY {
				return env.WriteJSON(output.NewResponse(s, fmt.Sprintf("Created: %s", s.Name)))
			}
			env.Status("Created global server: %s (%s)", s.Name, s.Identifier)
			return nil
		},
	}
	cmd.Flags().StringVar(&name, "name", "", "Server name (required)")
	cmd.Flags().StringVar(&protocol, "protocol", "", "Protocol type: ftp, sftp, ssh (required)")
	cmd.Flags().StringVar(&serverPath, "path", "", "Server path")
	cmd.Flags().StringVar(&environment, "environment", "", "Environment")
	return cmd
}

func newGlobalServersUpdateCmd() *cobra.Command {
	var name, protocol, serverPath, environment string
	cmd := &cobra.Command{
		Use: "update <id>", Short: "Update a global server", Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := cliCtx.Client()
			if err != nil {
				return err
			}
			s, err := client.UpdateGlobalServer(cliCtx.Background(), args[0], sdk.ServerUpdateRequest{
				Name: name, ProtocolType: protocol, ServerPath: serverPath, Environment: environment,
			})
			if err != nil {
				return err
			}
			cliCtx.Envelope.Status("Updated global server: %s", s.Name)
			return nil
		},
	}
	cmd.Flags().StringVar(&name, "name", "", "Server name")
	cmd.Flags().StringVar(&protocol, "protocol", "", "Protocol type")
	cmd.Flags().StringVar(&serverPath, "path", "", "Server path")
	cmd.Flags().StringVar(&environment, "environment", "", "Environment")
	return cmd
}

func newGlobalServersCopyCmd() *cobra.Command {
	var projectID string
	cmd := &cobra.Command{
		Use: "copy-to-project <server-id>", Short: "Copy a global server to a project", Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if projectID == "" {
				return &output.UserError{Message: "--to-project is required", Hint: "Specify which project to copy the server to"}
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
		newIntegrationsCreateCmd(),
		newIntegrationsUpdateCmd(),
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

func newIntegrationsCreateCmd() *cobra.Command {
	var hookType, name string
	cmd := &cobra.Command{
		Use: "create", Short: "Create an integration",
		RunE: func(cmd *cobra.Command, args []string) error {
			if hookType == "" {
				return &output.UserError{Message: "--type is required"}
			}
			projectID, err := cliCtx.RequireProject()
			if err != nil {
				return err
			}
			client, err := cliCtx.Client()
			if err != nil {
				return err
			}
			ig, err := client.CreateIntegration(cliCtx.Background(), projectID, sdk.IntegrationCreateRequest{
				HookType: hookType, Name: name,
			})
			if err != nil {
				return err
			}
			env := cliCtx.Envelope
			if env.JSONMode || !env.IsTTY {
				return env.WriteJSON(output.NewResponse(ig, fmt.Sprintf("Created: %s", ig.Name)))
			}
			env.Status("Created integration: %s (%s)", ig.Name, ig.Identifier)
			return nil
		},
	}
	cmd.Flags().StringVar(&hookType, "type", "", "Hook type (required)")
	cmd.Flags().StringVar(&name, "name", "", "Integration name")
	return cmd
}

func newIntegrationsUpdateCmd() *cobra.Command {
	var name string
	cmd := &cobra.Command{
		Use: "update <id>", Short: "Update an integration", Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			projectID, err := cliCtx.RequireProject()
			if err != nil {
				return err
			}
			client, err := cliCtx.Client()
			if err != nil {
				return err
			}
			ig, err := client.UpdateIntegration(cliCtx.Background(), projectID, args[0], sdk.IntegrationCreateRequest{
				Name: name,
			})
			if err != nil {
				return err
			}
			cliCtx.Envelope.Status("Updated integration: %s", ig.Name)
			return nil
		},
	}
	cmd.Flags().StringVar(&name, "name", "", "Integration name")
	return cmd
}
