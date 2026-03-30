package commands

import (
	"fmt"

	"github.com/deployhq/deployhq-cli/internal/output"
	"github.com/deployhq/deployhq-cli/pkg/sdk"
	"github.com/spf13/cobra"
)

func newConfigFilesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config-files",
		Short: "Manage config files",
	}
	cmd.AddCommand(
		newConfigFilesListCmd(),
		newConfigFilesShowCmd(),
		newConfigFilesCreateCmd(),
		newConfigFilesDeleteCmd(),
	)
	return cmd
}

func newConfigFilesListCmd() *cobra.Command {
	return &cobra.Command{
		Use: "list", Short: "List config files",
		RunE: func(cmd *cobra.Command, args []string) error {
			projectID, err := cliCtx.RequireProject()
			if err != nil {
				return err
			}
			client, err := cliCtx.Client()
			if err != nil {
				return err
			}
			files, err := client.ListConfigFiles(cliCtx.Background(), projectID)
			if err != nil {
				return err
			}
			env := cliCtx.Envelope
			if env.JSONMode || !env.IsTTY {
				return env.WriteJSON(output.NewResponse(files, fmt.Sprintf("%d config files", len(files))))
			}
			rows := make([][]string, len(files))
			for i, f := range files {
				rows[i] = []string{f.Identifier, f.Path, f.Description}
			}
			env.WriteTable([]string{"Identifier", "Path", "Description"}, rows)
			return nil
		},
	}
}

func newConfigFilesShowCmd() *cobra.Command {
	return &cobra.Command{
		Use: "show <id>", Short: "Show a config file", Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			projectID, err := cliCtx.RequireProject()
			if err != nil {
				return err
			}
			client, err := cliCtx.Client()
			if err != nil {
				return err
			}
			f, err := client.GetConfigFile(cliCtx.Background(), projectID, args[0])
			if err != nil {
				return err
			}
			env := cliCtx.Envelope
			if env.JSONMode || !env.IsTTY {
				return env.WriteJSON(output.NewResponse(f, f.Path))
			}
			env.WriteTable([]string{"Field", "Value"}, [][]string{
				{"Path", f.Path},
				{"Description", f.Description},
				{"Identifier", f.Identifier},
			})
			env.Status("\nContent:\n%s", f.Body)
			return nil
		},
	}
}

func newConfigFilesCreateCmd() *cobra.Command {
	var path, body, description string
	cmd := &cobra.Command{
		Use: "create", Short: "Create a config file",
		RunE: func(cmd *cobra.Command, args []string) error {
			if path == "" || body == "" {
				return &output.UserError{Message: "Both --path and --body are required"}
			}
			projectID, err := cliCtx.RequireProject()
			if err != nil {
				return err
			}
			client, err := cliCtx.Client()
			if err != nil {
				return err
			}
			f, err := client.CreateConfigFile(cliCtx.Background(), projectID, sdk.ConfigFileCreateRequest{
				Path: path, Body: body, Description: description,
			})
			if err != nil {
				return err
			}
			cliCtx.Envelope.Status("Created config file: %s", f.Path)
			return nil
		},
	}
	cmd.Flags().StringVar(&path, "path", "", "File path (required)")
	cmd.Flags().StringVar(&body, "body", "", "File content (required)")
	cmd.Flags().StringVar(&description, "description", "", "Description")
	return cmd
}

func newConfigFilesDeleteCmd() *cobra.Command {
	return &cobra.Command{
		Use: "delete <id>", Short: "Delete a config file", Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			projectID, err := cliCtx.RequireProject()
			if err != nil {
				return err
			}
			client, err := cliCtx.Client()
			if err != nil {
				return err
			}
			if err := client.DeleteConfigFile(cliCtx.Background(), projectID, args[0]); err != nil {
				return err
			}
			cliCtx.Envelope.Status("Deleted config file: %s", args[0])
			return nil
		},
	}
}
