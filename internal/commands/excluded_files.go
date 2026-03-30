package commands

import (
	"fmt"

	"github.com/deployhq/deployhq-cli/internal/output"
	"github.com/deployhq/deployhq-cli/pkg/sdk"
	"github.com/spf13/cobra"
)

func newExcludedFilesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "excluded-files",
		Short: "Manage excluded files",
	}
	cmd.AddCommand(
		&cobra.Command{
			Use: "list", Short: "List excluded files",
			RunE: func(cmd *cobra.Command, args []string) error {
				projectID, err := cliCtx.RequireProject()
				if err != nil {
					return err
				}
				client, err := cliCtx.Client()
				if err != nil {
					return err
				}
				files, err := client.ListExcludedFiles(cliCtx.Background(), projectID)
				if err != nil {
					return err
				}
				env := cliCtx.Envelope
				if env.JSONMode || !env.IsTTY {
					return env.WriteJSON(output.NewResponse(files, fmt.Sprintf("%d excluded files", len(files))))
				}
				rows := make([][]string, len(files))
				for i, f := range files {
					rows[i] = []string{f.Identifier, f.Path}
				}
				env.WriteTable([]string{"Identifier", "Path"}, rows)
				return nil
			},
		},
		newExcludedFilesCreateCmd(),
		&cobra.Command{
			Use: "delete <id>", Short: "Delete an excluded file", Args: cobra.ExactArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				projectID, err := cliCtx.RequireProject()
				if err != nil {
					return err
				}
				client, err := cliCtx.Client()
				if err != nil {
					return err
				}
				if err := client.DeleteExcludedFile(cliCtx.Background(), projectID, args[0]); err != nil {
					return err
				}
				cliCtx.Envelope.Status("Deleted excluded file: %s", args[0])
				return nil
			},
		},
	)
	return cmd
}

func newExcludedFilesCreateCmd() *cobra.Command {
	var path string
	cmd := &cobra.Command{
		Use: "create", Short: "Create an excluded file pattern",
		RunE: func(cmd *cobra.Command, args []string) error {
			if path == "" {
				return &output.UserError{Message: "--path is required"}
			}
			projectID, err := cliCtx.RequireProject()
			if err != nil {
				return err
			}
			client, err := cliCtx.Client()
			if err != nil {
				return err
			}
			f, err := client.CreateExcludedFile(cliCtx.Background(), projectID, sdk.ExcludedFileCreateRequest{Path: path})
			if err != nil {
				return err
			}
			cliCtx.Envelope.Status("Created excluded file: %s", f.Path)
			return nil
		},
	}
	cmd.Flags().StringVar(&path, "path", "", "File pattern to exclude (required)")
	return cmd
}
