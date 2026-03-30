package commands

import (
	"fmt"

	"github.com/deployhq/deployhq-cli/internal/output"
	"github.com/deployhq/deployhq-cli/pkg/sdk"
	"github.com/spf13/cobra"
)

func newServersCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "servers",
		Aliases: []string{"server", "srv"},
		Short:   "Manage servers",
	}

	cmd.AddCommand(
		newServersListCmd(),
		newServersShowCmd(),
		newServersCreateCmd(),
		newServersUpdateCmd(),
		newServersDeleteCmd(),
		newServersResetHostKeyCmd(),
	)

	return cmd
}

func newServersListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List servers in a project",
		RunE: func(cmd *cobra.Command, args []string) error {
			projectID, err := cliCtx.RequireProject()
			if err != nil {
				return err
			}

			client, err := cliCtx.Client()
			if err != nil {
				return err
			}

			servers, err := client.ListServers(cliCtx.Background(), projectID)
			if err != nil {
				return err
			}

			env := cliCtx.Envelope
			if env.JSONMode || !env.IsTTY {
				return env.WriteJSON(output.NewResponse(servers,
					fmt.Sprintf("%d servers", len(servers)),
					output.Breadcrumb{Action: "show", Cmd: fmt.Sprintf("deployhq servers show <id> -p %s", projectID)},
					output.Breadcrumb{Action: "deploy", Cmd: fmt.Sprintf("deployhq deploy -p %s", projectID)},
				))
			}

			columns := []string{"Name", "Identifier", "Protocol", "Branch", "Enabled"}
			rows := make([][]string, len(servers))
			for i, s := range servers {
				enabled := "yes"
				if !s.Enabled {
					enabled = "no"
				}
				rows[i] = []string{s.Name, s.Identifier, s.ProtocolType, s.Branch, enabled}
			}
			env.WriteTable(columns, rows)
			return nil
		},
	}
}

func newServersShowCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "show <identifier>",
		Short: "Show server details",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			projectID, err := cliCtx.RequireProject()
			if err != nil {
				return err
			}

			client, err := cliCtx.Client()
			if err != nil {
				return err
			}

			server, err := client.GetServer(cliCtx.Background(), projectID, args[0])
			if err != nil {
				return err
			}

			env := cliCtx.Envelope
			if env.JSONMode || !env.IsTTY {
				return env.WriteJSON(output.NewResponse(server,
					fmt.Sprintf("Server: %s", server.Name),
					output.Breadcrumb{Action: "deploy", Cmd: fmt.Sprintf("deployhq deploy -p %s", projectID)},
					output.Breadcrumb{Action: "reset-host-key", Cmd: fmt.Sprintf("deployhq servers reset-host-key %s -p %s", server.Identifier, projectID)},
				))
			}

			enabled := "yes"
			if !server.Enabled {
				enabled = "no"
			}
			env.WriteTable([]string{"Field", "Value"}, [][]string{
				{"Name", server.Name},
				{"Identifier", server.Identifier},
				{"Protocol", server.ProtocolType},
				{"Path", server.ServerPath},
				{"Branch", server.Branch},
				{"Environment", server.Environment},
				{"Enabled", enabled},
				{"Last Revision", server.LastRevision},
			})
			return nil
		},
	}
}

func newServersCreateCmd() *cobra.Command {
	var name, protocolType, serverPath, environment string

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a new server",
		RunE: func(cmd *cobra.Command, args []string) error {
			if name == "" {
				return &output.UserError{Message: "Server name is required", Hint: "Use --name flag"}
			}
			if protocolType == "" {
				return &output.UserError{Message: "Protocol type is required", Hint: "Use --protocol-type (ssh, ftp, sftp, s3, etc.)"}
			}

			projectID, err := cliCtx.RequireProject()
			if err != nil {
				return err
			}

			client, err := cliCtx.Client()
			if err != nil {
				return err
			}

			req := sdk.ServerCreateRequest{
				Name: name, ProtocolType: protocolType,
				ServerPath: serverPath, Environment: environment,
			}
			server, err := client.CreateServer(cliCtx.Background(), projectID, req)
			if err != nil {
				return err
			}

			env := cliCtx.Envelope
			if env.JSONMode || !env.IsTTY {
				return env.WriteJSON(output.NewResponse(server, fmt.Sprintf("Created server: %s", server.Name)))
			}
			env.Status("Created server: %s (%s)", server.Name, server.Identifier)
			return nil
		},
	}

	cmd.Flags().StringVar(&name, "name", "", "Server name (required)")
	cmd.Flags().StringVar(&protocolType, "protocol-type", "", "Protocol: ssh, ftp, sftp, s3, etc. (required)")
	cmd.Flags().StringVar(&serverPath, "path", "", "Server path")
	cmd.Flags().StringVar(&environment, "environment", "", "Environment name")
	return cmd
}

func newServersUpdateCmd() *cobra.Command {
	var name, serverPath, environment string

	cmd := &cobra.Command{
		Use:   "update <identifier>",
		Short: "Update a server",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			projectID, err := cliCtx.RequireProject()
			if err != nil {
				return err
			}

			client, err := cliCtx.Client()
			if err != nil {
				return err
			}

			req := sdk.ServerUpdateRequest{Name: name, ServerPath: serverPath, Environment: environment}
			server, err := client.UpdateServer(cliCtx.Background(), projectID, args[0], req)
			if err != nil {
				return err
			}

			env := cliCtx.Envelope
			if env.JSONMode || !env.IsTTY {
				return env.WriteJSON(output.NewResponse(server, fmt.Sprintf("Updated server: %s", server.Name)))
			}
			env.Status("Updated server: %s", server.Name)
			return nil
		},
	}

	cmd.Flags().StringVar(&name, "name", "", "Server name")
	cmd.Flags().StringVar(&serverPath, "path", "", "Server path")
	cmd.Flags().StringVar(&environment, "environment", "", "Environment name")
	return cmd
}

func newServersDeleteCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "delete <identifier>",
		Short: "Delete a server",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			projectID, err := cliCtx.RequireProject()
			if err != nil {
				return err
			}

			client, err := cliCtx.Client()
			if err != nil {
				return err
			}

			if err := client.DeleteServer(cliCtx.Background(), projectID, args[0]); err != nil {
				return err
			}
			cliCtx.Envelope.Status("Deleted server: %s", args[0])
			return nil
		},
	}
}

func newServersResetHostKeyCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "reset-host-key <identifier>",
		Short: "Reset SSH host key for a server",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			projectID, err := cliCtx.RequireProject()
			if err != nil {
				return err
			}

			client, err := cliCtx.Client()
			if err != nil {
				return err
			}

			if err := client.ResetServerHostKey(cliCtx.Background(), projectID, args[0]); err != nil {
				return err
			}
			cliCtx.Envelope.Status("Reset host key for server: %s", args[0])
			return nil
		},
	}
}
