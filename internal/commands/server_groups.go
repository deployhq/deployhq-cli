package commands

import (
	"fmt"
	"strings"

	"github.com/deployhq/deployhq-cli/internal/output"
	"github.com/deployhq/deployhq-cli/pkg/sdk"
	"github.com/spf13/cobra"
)

func newServerGroupsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "server-groups",
		Aliases: []string{"sg"},
		Short:   "Manage server groups",
		Long: `Logical groupings of servers — typically by environment (production, staging) or role (web, worker). Deploying to a group deploys to every server in it.

The group's transfer order decides how: sequential runs each server's whole deployment as one unit, parallel finishes each step on every server before any server starts the next step.

Useful for clusters and blue-green setups where one logical "deploy" actually targets many machines.`,
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

// transferOrders are the values the DeployHQ backend accepts for a server
// group's transfer_order. Anything else is refused with a 422, so the CLI
// rejects it locally first.
var transferOrders = []string{"sequential", "parallel"}

// transferOrderUsage is the --transfer-order help shared by the project and
// template server-group commands, so the two read alike.
const transferOrderUsage = "Transfer order: sequential (each server's whole deployment runs as one unit) " +
	"or parallel (each step finishes on every server before any server starts the next step)"

// validateTransferOrder rejects a supplied --transfer-order the backend would
// refuse. An omitted flag passes: nothing is sent, so the backend's default
// applies. It runs before any project or client resolution, so a malformed
// invocation fails with no network access.
func validateTransferOrder(cmd *cobra.Command, value string) error {
	if !cmd.Flags().Changed("transfer-order") {
		return nil
	}
	for _, v := range transferOrders {
		if value == v {
			return nil
		}
	}
	return &output.UserError{
		Message: fmt.Sprintf("Invalid --transfer-order %q", value),
		Hint:    "Use one of: " + strings.Join(transferOrders, ", "),
	}
}

func newServerGroupsListCmd() *cobra.Command {
	var page, perPage int

	cmd := &cobra.Command{
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

			groups, err := client.ListServerGroups(cliCtx.Background(), projectID, listOptsFromFlags(page, perPage))
			if err != nil {
				return err
			}

			env := cliCtx.Envelope
			if env.WantsJSON() {
				return env.WriteJSON(output.NewResponse(groups, fmt.Sprintf("%d server groups", len(groups))))
			}

			if env.QuietMode {
				identifiers := make([]string, len(groups))
				for i, g := range groups {
					identifiers[i] = g.Identifier
				}
				env.WriteQuiet(identifiers)
				return nil
			}

			columns := []string{"Name", "Identifier", "Servers", "Environment"}
			rows := make([][]string, len(groups))
			for i, g := range groups {
				rows[i] = []string{g.Name, g.Identifier, fmt.Sprintf("%d", len(g.Servers)), g.Environment}
			}
			env.WriteTable(columns, rows)

			if len(groups) > 0 {
				env.Status("\nTip: dhq server-groups show %s -p %s", groups[0].Identifier, projectID)
			}
			return nil
		},
	}

	addPaginationFlags(cmd, &page, &perPage)
	return cmd
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
			if env.WantsJSON() {
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

			env.Status("\nNext commands:")
			env.Status("  dhq deploy -p %s", projectID)
			env.Status("  dhq servers list -p %s", projectID)
			env.Status("  dhq server-groups update %s -p %s", group.Identifier, projectID)
			return nil
		},
	}
}

func newServerGroupsCreateCmd() *cobra.Command {
	var name, transferOrder string

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a server group",
		Long: `Create a server group in a project.

Omit --transfer-order to take DeployHQ's default (sequential).`,
		Example: `  dhq server-groups create -p my-app --name "Web"
  dhq server-groups create -p my-app --name "Web" --transfer-order parallel`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if name == "" {
				return &output.UserError{Message: "Name is required", Hint: "Use --name flag"}
			}
			if err := validateTransferOrder(cmd, transferOrder); err != nil {
				return err
			}

			projectID, err := cliCtx.RequireProject()
			if err != nil {
				return err
			}

			client, err := cliCtx.Client()
			if err != nil {
				return err
			}

			group, err := client.CreateServerGroup(cliCtx.Background(), projectID, sdk.ServerGroupCreateRequest{
				Name:          name,
				TransferOrder: transferOrder,
			})
			if err != nil {
				return err
			}

			env := cliCtx.Envelope
			if env.WantsJSON() {
				return env.WriteJSON(output.NewResponse(group, fmt.Sprintf("Created server group: %s", group.Name)))
			}
			env.Status("Created server group: %s (%s)", group.Name, group.Identifier)
			return nil
		},
	}

	cmd.Flags().StringVar(&name, "name", "", "Server group name (required)")
	cmd.Flags().StringVar(&transferOrder, "transfer-order", "", transferOrderUsage)
	return cmd
}

func newServerGroupsUpdateCmd() *cobra.Command {
	var name, transferOrder string

	cmd := &cobra.Command{
		Use:   "update <identifier>",
		Short: "Update a server group",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateTransferOrder(cmd, transferOrder); err != nil {
				return err
			}

			projectID, err := cliCtx.RequireProject()
			if err != nil {
				return err
			}

			client, err := cliCtx.Client()
			if err != nil {
				return err
			}

			group, err := client.UpdateServerGroup(cliCtx.Background(), projectID, args[0], sdk.ServerGroupUpdateRequest{
				Name:          name,
				TransferOrder: transferOrder,
			})
			if err != nil {
				return err
			}

			env := cliCtx.Envelope
			if env.WantsJSON() {
				return env.WriteJSON(output.NewResponse(group, fmt.Sprintf("Updated server group: %s", group.Name)))
			}
			env.Status("Updated server group: %s", group.Name)
			return nil
		},
	}

	cmd.Flags().StringVar(&name, "name", "", "New name")
	cmd.Flags().StringVar(&transferOrder, "transfer-order", "", transferOrderUsage)
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
