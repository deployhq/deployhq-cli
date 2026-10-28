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
		Long: `File or directory patterns to skip when uploading a deployment. Common uses: keeping local-only files (.env.local, .DS_Store, .git) off production servers, or pruning documentation and tests from the deployed artifact.

Patterns are evaluated against repository paths and apply per-project. Each excluded-file entry may target specific servers or all of them.`,
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
				files, err := client.ListExcludedFiles(cliCtx.Background(), projectID, nil)
				if err != nil {
					return err
				}
				env := cliCtx.Envelope
				if env.WantsJSON() {
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
		&cobra.Command{
			Use: "show <id>", Short: "Show an excluded file", Args: cobra.ExactArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				projectID, err := cliCtx.RequireProject()
				if err != nil {
					return err
				}
				client, err := cliCtx.Client()
				if err != nil {
					return err
				}
				f, err := client.GetExcludedFile(cliCtx.Background(), projectID, args[0])
				if err != nil {
					return err
				}
				return cliCtx.Envelope.WriteJSON(output.NewResponse(f, f.Path))
			},
		},
		newExcludedFilesCreateCmd(),
		newExcludedFilesUpdateCmd(),
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

func newExcludedFilesUpdateCmd() *cobra.Command {
	var path string
	var servers []string
	var allServers bool
	cmd := &cobra.Command{
		Use: "update <id>", Short: "Update an excluded file pattern", Args: cobra.ExactArgs(1),
		Example: `  # Change the pattern
  dhq excluded-files update <id> --path "*.log" -p my-app

  # Make an existing rule apply to every server
  dhq excluded-files update <id> --all-servers -p my-app`,
		RunE: func(cmd *cobra.Command, args []string) error {
			pathSet := cmd.Flags().Changed("path")
			serversSet := cmd.Flags().Changed("server")
			if !pathSet && !serversSet && !allServers {
				return &output.UserError{
					Message: "Nothing to update",
					Hint:    "Pass --path, --server or --all-servers",
				}
			}
			if pathSet && path == "" {
				return &output.UserError{Message: "--path cannot be empty"}
			}
			projectID, err := cliCtx.RequireProject()
			if err != nil {
				return err
			}
			client, err := cliCtx.Client()
			if err != nil {
				return err
			}
			// Only send what the user asked to change: omitting both server
			// fields leaves the rule's current targeting as it is.
			req := sdk.ExcludedFileCreateRequest{Path: path, AllServers: allServers}
			if serversSet {
				ids, err := resolveServerTargets(cliCtx.Background(), client, projectID, servers)
				if err != nil {
					return err
				}
				req.ServerIdentifiers = ids
			}
			f, err := client.UpdateExcludedFile(cliCtx.Background(), projectID, args[0], req)
			if err != nil {
				return err
			}
			cliCtx.Envelope.Status("Updated excluded file: %s", f.Path)
			return nil
		},
	}
	cmd.Flags().StringVar(&path, "path", "", "File pattern")
	addServerFlag(cmd, &servers)
	addAllServersFlag(cmd, &allServers)
	return cmd
}

func newExcludedFilesCreateCmd() *cobra.Command {
	var path string
	var servers []string
	cmd := &cobra.Command{
		Use: "create", Short: "Create an excluded file pattern",
		Long: `Create a pattern for files DeployHQ should not upload.

The rule applies to all servers in the project (including servers added later)
unless --server names specific servers or server groups.`,
		Example: `  # Never upload deploy.sh, on any server
  dhq excluded-files create --path deploy.sh -p my-app

  # Only skip it on one server
  dhq excluded-files create --path deploy.sh --server production -p my-app`,
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
			req := sdk.ExcludedFileCreateRequest{Path: path}
			if cmd.Flags().Changed("server") {
				ids, err := resolveServerTargets(cliCtx.Background(), client, projectID, servers)
				if err != nil {
					return err
				}
				req.ServerIdentifiers = ids
			} else {
				req.AllServers = true
			}
			f, err := client.CreateExcludedFile(cliCtx.Background(), projectID, req)
			if err != nil {
				return err
			}
			env := cliCtx.Envelope
			if env.WantsJSON() {
				return env.WriteJSON(output.NewResponse(f,
					fmt.Sprintf("Created excluded file: %s", f.Path),
					output.Breadcrumb{Action: "show", Cmd: fmt.Sprintf("dhq excluded-files show %s -p %s", f.Identifier, projectID)},
				))
			}
			env.Status("Created excluded file: %s (%s)", f.Path, describeServerTargets(req.ServerIdentifiers))
			return nil
		},
	}
	cmd.Flags().StringVar(&path, "path", "", "File pattern to exclude (required)")
	addServerFlag(cmd, &servers)
	return cmd
}
