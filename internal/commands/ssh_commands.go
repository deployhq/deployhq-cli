package commands

import (
	"fmt"

	"github.com/deployhq/deployhq-cli/internal/output"
	"github.com/deployhq/deployhq-cli/pkg/sdk"
	"github.com/spf13/cobra"
)

func newSSHCommandsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "ssh-commands",
		Short: "Manage SSH commands",
		Long: `Commands run over SSH on the deployment target — restart services, run migrations, warm caches, etc. Each command pins to a phase relative to the file upload (pre-upload, post-upload) and may target specific servers.

Build commands run on the build server; SSH commands run on the deploy target.`,
	}
	cmd.AddCommand(
		&cobra.Command{
			Use: "list", Short: "List SSH commands",
			RunE: func(cmd *cobra.Command, args []string) error {
				projectID, err := cliCtx.RequireProject()
				if err != nil {
					return err
				}
				client, err := cliCtx.Client()
				if err != nil {
					return err
				}
				cmds, err := client.ListSSHCommands(cliCtx.Background(), projectID, nil)
				if err != nil {
					return err
				}
				env := cliCtx.Envelope
				if env.WantsJSON() {
					return env.WriteJSON(output.NewResponse(cmds, fmt.Sprintf("%d SSH commands", len(cmds))))
				}
				rows := make([][]string, len(cmds))
				for i, c := range cmds {
					rows[i] = []string{c.Identifier, c.Command, c.Timing, c.Description}
				}
				env.WriteTable([]string{"Identifier", "Command", "Timing", "Description"}, rows)
				return nil
			},
		},
		&cobra.Command{
			Use: "show <id>", Short: "Show SSH command details", Args: cobra.ExactArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				projectID, err := cliCtx.RequireProject()
				if err != nil {
					return err
				}
				client, err := cliCtx.Client()
				if err != nil {
					return err
				}
				c, err := client.GetSSHCommand(cliCtx.Background(), projectID, args[0])
				if err != nil {
					return err
				}
				return cliCtx.Envelope.WriteJSON(output.NewResponse(c, c.Description))
			},
		},
		newSSHCommandsCreateCmd(),
		newSSHCommandsUpdateCmd(),
		&cobra.Command{
			Use: "delete <id>", Short: "Delete an SSH command", Args: cobra.ExactArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				projectID, err := cliCtx.RequireProject()
				if err != nil {
					return err
				}
				client, err := cliCtx.Client()
				if err != nil {
					return err
				}
				if err := client.DeleteSSHCommand(cliCtx.Background(), projectID, args[0]); err != nil {
					return err
				}
				cliCtx.Envelope.Status("Deleted SSH command: %s", args[0])
				return nil
			},
		},
	)
	return cmd
}

// validSSHTimings mirrors Command::TIMING in the DeployHQ Rails model.
var validSSHTimings = []string{"all", "first", "after_first"}

func validateSSHTiming(timing string) error {
	if timing == "" {
		return nil
	}
	for _, t := range validSSHTimings {
		if timing == t {
			return nil
		}
	}
	return &output.UserError{
		Message: fmt.Sprintf("Invalid --timing %q", timing),
		Hint:    "Valid values: all, first, after_first",
	}
}

func newSSHCommandsUpdateCmd() *cobra.Command {
	var command, description, timing string
	cmd := &cobra.Command{
		Use: "update <id>", Short: "Update an SSH command", Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateSSHTiming(timing); err != nil {
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
			c, err := client.UpdateSSHCommand(cliCtx.Background(), projectID, args[0], sdk.SSHCommandCreateRequest{
				Command: command, Description: description, Timing: timing,
			})
			if err != nil {
				return err
			}
			cliCtx.Envelope.Status("Updated SSH command: %s", c.Identifier)
			return nil
		},
	}
	cmd.Flags().StringVar(&command, "command", "", "SSH command")
	cmd.Flags().StringVar(&description, "description", "", "Description")
	cmd.Flags().StringVar(&timing, "timing", "", "Timing: all, first, or after_first")
	return cmd
}

func newSSHCommandsCreateCmd() *cobra.Command {
	var command, description, timing string
	cmd := &cobra.Command{
		Use: "create", Short: "Create an SSH command",
		RunE: func(cmd *cobra.Command, args []string) error {
			if command == "" {
				return &output.UserError{Message: "--command is required"}
			}
			if err := validateSSHTiming(timing); err != nil {
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
			c, err := client.CreateSSHCommand(cliCtx.Background(), projectID, sdk.SSHCommandCreateRequest{
				Command: command, Description: description, Timing: timing,
			})
			if err != nil {
				return err
			}
			cliCtx.Envelope.Status("Created SSH command: %s", c.Identifier)
			return nil
		},
	}
	cmd.Flags().StringVar(&command, "command", "", "SSH command (required)")
	cmd.Flags().StringVar(&description, "description", "", "Description")
	cmd.Flags().StringVar(&timing, "timing", "all", "Timing: all, first, or after_first")
	return cmd
}
