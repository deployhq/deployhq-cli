package commands

import (
	"fmt"

	"github.com/deployhq/deployhq-cli/internal/output"
	"github.com/deployhq/deployhq-cli/pkg/sdk"
	"github.com/spf13/cobra"
)

func newBuildCommandsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "build-commands",
		Short: "Manage build commands",
	}
	cmd.AddCommand(
		&cobra.Command{
			Use: "list", Short: "List build commands",
			RunE: func(cmd *cobra.Command, args []string) error {
				projectID, err := cliCtx.RequireProject()
				if err != nil {
					return err
				}
				client, err := cliCtx.Client()
				if err != nil {
					return err
				}
				cmds, err := client.ListBuildCommands(cliCtx.Background(), projectID)
				if err != nil {
					return err
				}
				env := cliCtx.Envelope
				if env.JSONMode || !env.IsTTY {
					return env.WriteJSON(output.NewResponse(cmds, fmt.Sprintf("%d build commands", len(cmds))))
				}
				rows := make([][]string, len(cmds))
				for i, c := range cmds {
					enabled := "yes"
					if !c.Enabled {
						enabled = "no"
					}
					rows[i] = []string{c.Identifier, c.Command, c.Description, enabled}
				}
				env.WriteTable([]string{"Identifier", "Command", "Description", "Enabled"}, rows)
				return nil
			},
		},
		newBuildCommandsCreateCmd(),
		&cobra.Command{
			Use: "delete <id>", Short: "Delete a build command", Args: cobra.ExactArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				projectID, err := cliCtx.RequireProject()
				if err != nil {
					return err
				}
				client, err := cliCtx.Client()
				if err != nil {
					return err
				}
				if err := client.DeleteBuildCommand(cliCtx.Background(), projectID, args[0]); err != nil {
					return err
				}
				cliCtx.Envelope.Status("Deleted build command: %s", args[0])
				return nil
			},
		},
	)
	return cmd
}

func newBuildCommandsCreateCmd() *cobra.Command {
	var command, description string
	cmd := &cobra.Command{
		Use: "create", Short: "Create a build command",
		RunE: func(cmd *cobra.Command, args []string) error {
			if command == "" {
				return &output.UserError{Message: "--command is required"}
			}
			projectID, err := cliCtx.RequireProject()
			if err != nil {
				return err
			}
			client, err := cliCtx.Client()
			if err != nil {
				return err
			}
			c, err := client.CreateBuildCommand(cliCtx.Background(), projectID, sdk.BuildCommandCreateRequest{
				Command: command, Description: description,
			})
			if err != nil {
				return err
			}
			cliCtx.Envelope.Status("Created build command: %s", c.Identifier)
			return nil
		},
	}
	cmd.Flags().StringVar(&command, "command", "", "Build command (required)")
	cmd.Flags().StringVar(&description, "description", "", "Description")
	return cmd
}
