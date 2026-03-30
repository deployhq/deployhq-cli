package commands

import (
	"fmt"

	"github.com/deployhq/deployhq-cli/internal/output"
	"github.com/deployhq/deployhq-cli/pkg/sdk"
	"github.com/spf13/cobra"
)

func newServerGroupsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "server-groups",
		Aliases: []string{"sg"},
		Short:   "Manage server groups",
	}

	cmd.AddCommand(
		newServerGroupsListCmd(),
		newServerGroupsShowCmd(),
		newServerGroupsCreateCmd(),
		newServerGroupsUpdateCmd(),
		newServerGroupsDeleteCmd(),
	)

	return cmd
}

func newServerGroupsListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List server groups",
		RunE: func(cmd *cobra.Command, args []string) error {
			projectID, err := cliCtx.RequireProject()
			if err != nil {
				return err
			}

			client, err := cliCtx.Client()
			if err != nil {
				return err
			}

			groups, err := client.ListServerGroups(cliCtx.Background(), projectID)
			if err != nil {
				return err
			}

			env := cliCtx.Envelope
			if env.JSONMode || !env.IsTTY {
				return env.WriteJSON(output.NewResponse(groups, fmt.Sprintf("%d server groups", len(groups))))
			}

			columns := []string{"Name", "Identifier", "Servers", "Environment"}
			rows := make([][]string, len(groups))
			for i, g := range groups {
				rows[i] = []string{g.Name, g.Identifier, fmt.Sprintf("%d", len(g.Servers)), g.Environment}
			}
			env.WriteTable(columns, rows)
			return nil
		},
	}
}

func newServerGroupsShowCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "show <identifier>",
		Short: "Show server group details",
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

			group, err := client.GetServerGroup(cliCtx.Background(), projectID, args[0])
			if err != nil {
				return err
			}

			env := cliCtx.Envelope
			if env.JSONMode || !env.IsTTY {
				return env.WriteJSON(output.NewResponse(group, fmt.Sprintf("Server group: %s", group.Name)))
			}

			env.WriteTable([]string{"Field", "Value"}, [][]string{
				{"Name", group.Name},
				{"Identifier", group.Identifier},
				{"Servers", fmt.Sprintf("%d", len(group.Servers))},
				{"Environment", group.Environment},
			})

			if len(group.Servers) > 0 {
				env.Status("\nServers:")
				srvCols := []string{"Name", "Identifier", "Protocol"}
				srvRows := make([][]string, len(group.Servers))
				for i, s := range group.Servers {
					srvRows[i] = []string{s.Name, s.Identifier, s.ProtocolType}
				}
				env.WriteTable(srvCols, srvRows)
			}
			return nil
		},
	}
}

func newServerGroupsCreateCmd() *cobra.Command {
	var name string

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a server group",
		RunE: func(cmd *cobra.Command, args []string) error {
			if name == "" {
				return &output.UserError{Message: "Name is required", Hint: "Use --name flag"}
			}

			projectID, err := cliCtx.RequireProject()
			if err != nil {
				return err
			}

			client, err := cliCtx.Client()
			if err != nil {
				return err
			}

			group, err := client.CreateServerGroup(cliCtx.Background(), projectID, sdk.ServerGroupCreateRequest{Name: name})
			if err != nil {
				return err
			}

			env := cliCtx.Envelope
			if env.JSONMode || !env.IsTTY {
				return env.WriteJSON(output.NewResponse(group, fmt.Sprintf("Created server group: %s", group.Name)))
			}
			env.Status("Created server group: %s (%s)", group.Name, group.Identifier)
			return nil
		},
	}

	cmd.Flags().StringVar(&name, "name", "", "Server group name (required)")
	return cmd
}

func newServerGroupsUpdateCmd() *cobra.Command {
	var name string

	cmd := &cobra.Command{
		Use:   "update <identifier>",
		Short: "Update a server group",
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

			group, err := client.UpdateServerGroup(cliCtx.Background(), projectID, args[0], sdk.ServerGroupUpdateRequest{Name: name})
			if err != nil {
				return err
			}

			env := cliCtx.Envelope
			if env.JSONMode || !env.IsTTY {
				return env.WriteJSON(output.NewResponse(group, fmt.Sprintf("Updated server group: %s", group.Name)))
			}
			env.Status("Updated server group: %s", group.Name)
			return nil
		},
	}

	cmd.Flags().StringVar(&name, "name", "", "New name")
	return cmd
}

func newServerGroupsDeleteCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "delete <identifier>",
		Short: "Delete a server group",
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

			if err := client.DeleteServerGroup(cliCtx.Background(), projectID, args[0]); err != nil {
				return err
			}
			cliCtx.Envelope.Status("Deleted server group: %s", args[0])
			return nil
		},
	}
}
