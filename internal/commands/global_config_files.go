package commands

import (
	"fmt"

	"github.com/deployhq/deployhq-cli/internal/output"
	"github.com/deployhq/deployhq-cli/pkg/sdk"
	"github.com/spf13/cobra"
)

func newGlobalConfigFilesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "global-config-files",
		Short: "Manage global config files",
	}
	cmd.AddCommand(
		&cobra.Command{
			Use: "list", Short: "List global config files",
			RunE: func(cmd *cobra.Command, args []string) error {
				client, err := cliCtx.Client()
				if err != nil {
					return err
				}
				files, err := client.ListGlobalConfigFiles(cliCtx.Background())
				if err != nil {
					return err
				}
				env := cliCtx.Envelope
				if env.JSONMode || !env.IsTTY {
					return env.WriteJSON(output.NewResponse(files, fmt.Sprintf("%d global config files", len(files))))
				}
				rows := make([][]string, len(files))
				for i, f := range files {
					rows[i] = []string{f.Identifier, f.Name}
				}
				env.WriteTable([]string{"Identifier", "Name"}, rows)
				return nil
			},
		},
		&cobra.Command{
			Use: "show <id>", Short: "Show a global config file", Args: cobra.ExactArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				client, err := cliCtx.Client()
				if err != nil {
					return err
				}
				f, err := client.GetGlobalConfigFile(cliCtx.Background(), args[0])
				if err != nil {
					return err
				}
				env := cliCtx.Envelope
				if env.JSONMode || !env.IsTTY {
					return env.WriteJSON(output.NewResponse(f, f.Name))
				}
				env.WriteTable([]string{"Field", "Value"}, [][]string{
					{"Name", f.Name},
					{"Identifier", f.Identifier},
				})
				env.Status("\nContent:\n%s", f.Body)
				return nil
			},
		},
		newGlobalConfigFilesCreateCmd(),
		newGlobalConfigFilesUpdateCmd(),
		&cobra.Command{
			Use: "delete <id>", Short: "Delete a global config file", Args: cobra.ExactArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				client, err := cliCtx.Client()
				if err != nil {
					return err
				}
				if err := client.DeleteGlobalConfigFile(cliCtx.Background(), args[0]); err != nil {
					return err
				}
				cliCtx.Envelope.Status("Deleted global config file: %s", args[0])
				return nil
			},
		},
	)
	return cmd
}

func newGlobalConfigFilesCreateCmd() *cobra.Command {
	var name, body string
	cmd := &cobra.Command{
		Use: "create", Short: "Create a global config file",
		RunE: func(cmd *cobra.Command, args []string) error {
			if name == "" || body == "" {
				return &output.UserError{Message: "Both --name and --body are required"}
			}
			client, err := cliCtx.Client()
			if err != nil {
				return err
			}
			f, err := client.CreateGlobalConfigFile(cliCtx.Background(), sdk.GlobalConfigFileCreateRequest{
				Name: name, Body: body,
			})
			if err != nil {
				return err
			}
			env := cliCtx.Envelope
			if env.JSONMode || !env.IsTTY {
				return env.WriteJSON(output.NewResponse(f, fmt.Sprintf("Created: %s", f.Name)))
			}
			env.Status("Created global config file: %s (%s)", f.Name, f.Identifier)
			return nil
		},
	}
	cmd.Flags().StringVar(&name, "name", "", "File name (required)")
	cmd.Flags().StringVar(&body, "body", "", "File content (required)")
	return cmd
}

func newGlobalConfigFilesUpdateCmd() *cobra.Command {
	var name, body string
	cmd := &cobra.Command{
		Use: "update <id>", Short: "Update a global config file", Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := cliCtx.Client()
			if err != nil {
				return err
			}
			f, err := client.UpdateGlobalConfigFile(cliCtx.Background(), args[0], sdk.GlobalConfigFileCreateRequest{
				Name: name, Body: body,
			})
			if err != nil {
				return err
			}
			cliCtx.Envelope.Status("Updated global config file: %s", f.Name)
			return nil
		},
	}
	cmd.Flags().StringVar(&name, "name", "", "File name")
	cmd.Flags().StringVar(&body, "body", "", "File content")
	return cmd
}
