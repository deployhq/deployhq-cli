package commands

import (
	"fmt"

	"github.com/deployhq/deployhq-cli/internal/output"
	"github.com/deployhq/deployhq-cli/pkg/sdk"
	"github.com/spf13/cobra"
)

func newBuildCacheFilesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "build-cache-files",
		Short: "Manage build cache files",
	}
	cmd.AddCommand(
		&cobra.Command{
			Use: "list", Short: "List build cache files",
			RunE: func(cmd *cobra.Command, args []string) error {
				projectID, err := cliCtx.RequireProject()
				if err != nil {
					return err
				}
				client, err := cliCtx.Client()
				if err != nil {
					return err
				}
				files, err := client.ListBuildCacheFiles(cliCtx.Background(), projectID)
				if err != nil {
					return err
				}
				env := cliCtx.Envelope
				if env.JSONMode || !env.IsTTY {
					return env.WriteJSON(output.NewResponse(files, fmt.Sprintf("%d build cache files", len(files))))
				}
				rows := make([][]string, len(files))
				for i, f := range files {
					rows[i] = []string{f.Identifier, f.Path}
				}
				env.WriteTable([]string{"Identifier", "Path"}, rows)
				return nil
			},
		},
		newBuildCacheFilesCreateCmd(),
		newBuildCacheFilesUpdateCmd(),
		&cobra.Command{
			Use: "delete <id>", Short: "Delete a build cache file", Args: cobra.ExactArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				projectID, err := cliCtx.RequireProject()
				if err != nil {
					return err
				}
				client, err := cliCtx.Client()
				if err != nil {
					return err
				}
				if err := client.DeleteBuildCacheFile(cliCtx.Background(), projectID, args[0]); err != nil {
					return err
				}
				cliCtx.Envelope.Status("Deleted build cache file: %s", args[0])
				return nil
			},
		},
	)
	return cmd
}

func newBuildCacheFilesCreateCmd() *cobra.Command {
	var path string
	cmd := &cobra.Command{
		Use: "create", Short: "Create a build cache file",
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
			f, err := client.CreateBuildCacheFile(cliCtx.Background(), projectID, sdk.BuildCacheFileCreateRequest{
				Path: path,
			})
			if err != nil {
				return err
			}
			env := cliCtx.Envelope
			if env.JSONMode || !env.IsTTY {
				return env.WriteJSON(output.NewResponse(f, fmt.Sprintf("Created: %s", f.Path)))
			}
			env.Status("Created build cache file: %s", f.Path)
			return nil
		},
	}
	cmd.Flags().StringVar(&path, "path", "", "Cache file path (required)")
	return cmd
}

func newBuildCacheFilesUpdateCmd() *cobra.Command {
	var path string
	cmd := &cobra.Command{
		Use: "update <id>", Short: "Update a build cache file", Args: cobra.ExactArgs(1),
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
			f, err := client.UpdateBuildCacheFile(cliCtx.Background(), projectID, args[0], sdk.BuildCacheFileCreateRequest{
				Path: path,
			})
			if err != nil {
				return err
			}
			cliCtx.Envelope.Status("Updated build cache file: %s", f.Path)
			return nil
		},
	}
	cmd.Flags().StringVar(&path, "path", "", "Cache file path (required)")
	return cmd
}
