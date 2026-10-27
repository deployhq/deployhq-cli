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
		Long: `Per-project config file overrides DeployHQ writes onto each server during a deploy. Useful for environment-specific secrets and configuration that aren't (and shouldn't be) in source control — production database URLs, API keys, .env files.

Each config file is scoped to one project and pinned to a target path on the server. For account-wide files reusable across projects, see "dhq global-config-files".`,
	}
	cmd.AddCommand(
		newConfigFilesListCmd(),
		newConfigFilesShowCmd(),
		newConfigFilesCreateCmd(),
		newConfigFilesUpdateCmd(),
		newConfigFilesDeleteCmd(),
		newConfigFilesLinkGlobalCmd(),
		newConfigFilesUnlinkGlobalCmd(),
	)
	return cmd
}

func newConfigFilesLinkGlobalCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "link-global <config-file-id>",
		Short: "Link a global config file into this project",
		Args:  cobra.ExactArgs(1),
		Example: `  # Link an account-wide config file into the current project
  dhq config-files link-global cfg-abc123 -p my-app`,
		RunE: func(cmd *cobra.Command, args []string) error {
			projectID, err := cliCtx.RequireProject()
			if err != nil {
				return err
			}
			client, err := cliCtx.Client()
			if err != nil {
				return err
			}
			f, err := client.LinkGlobalConfigFile(cliCtx.Background(), projectID, args[0])
			if err != nil {
				return err
			}
			env := cliCtx.Envelope
			if env.WantsJSON() {
				return env.WriteJSON(output.NewResponse(f, fmt.Sprintf("Linked global config file: %s", f.Path)))
			}
			env.Status("Linked global config file: %s", f.Path)
			return nil
		},
	}
}

func newConfigFilesUnlinkGlobalCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "unlink-global <config-file-id>",
		Short: "Unlink a global config file from this project",
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
			if err := client.UnlinkGlobalConfigFile(cliCtx.Background(), projectID, args[0]); err != nil {
				return err
			}
			env := cliCtx.Envelope
			if env.WantsJSON() {
				return env.WriteJSON(output.NewResponse(
					map[string]string{"config_file_id": args[0], "status": "unlinked"},
					fmt.Sprintf("Unlinked: %s", args[0])))
			}
			env.Status("Unlinked global config file: %s", args[0])
			return nil
		},
	}
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
			files, err := client.ListConfigFiles(cliCtx.Background(), projectID, nil)
			if err != nil {
				return err
			}
			env := cliCtx.Envelope
			if env.WantsJSON() {
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
			if env.WantsJSON() {
				return env.WriteJSON(output.NewResponse(f, f.Path))
			}
			env.WriteTable([]string{"Field", "Value"}, [][]string{
				{"Path", f.Path},
				{"Description", f.Description},
				{"Identifier", f.Identifier},
			})
			env.Status("\nContent:\n%s", f.Body)

			env.Status("\nTip: dhq config-files update %s -p %s", f.Identifier, projectID)
			return nil
		},
	}
}

func newConfigFilesCreateCmd() *cobra.Command {
	var path, body, description string
	var servers []string
	var buildOnly bool
	cmd := &cobra.Command{
		Use: "create", Short: "Create a config file",
		Long: `Create a config file that DeployHQ writes during each deployment.

The file is uploaded to all servers in the project (including servers added
later) unless --server names specific servers or server groups. Use
--build-only for a file that is only available to build commands and never
uploaded to a server (for example package-manager credentials).`,
		Example: `  # Upload .env to every server
  dhq config-files create --path .env --body "APP_ENV=production" -p my-app

  # Only upload it to one server
  dhq config-files create --path .env --body "APP_ENV=production" --server production -p my-app

  # Make a file available to the build only
  dhq config-files create --path .npmrc --body "//registry.npmjs.org/:_authToken=..." --build-only -p my-app`,
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
			req := sdk.ConfigFileCreateRequest{Path: path, Body: body, Description: description}
			target := "all servers"
			switch {
			case buildOnly:
				// No server targeting at all is how DeployHQ stores a
				// build-only file.
				build := true
				req.Build = &build
				target = "build only"
			case cmd.Flags().Changed("server"):
				ids, err := resolveServerTargets(cliCtx.Background(), client, projectID, servers)
				if err != nil {
					return err
				}
				req.ServerIdentifiers = ids
				target = describeServerTargets(ids)
			default:
				req.AllServers = true
			}
			f, err := client.CreateConfigFile(cliCtx.Background(), projectID, req)
			if err != nil {
				return err
			}
			cliCtx.Envelope.Status("Created config file: %s (%s)", f.Path, target)
			return nil
		},
	}
	cmd.Flags().StringVar(&path, "path", "", "File path (required)")
	cmd.Flags().StringVar(&body, "body", "", "File content (required)")
	cmd.Flags().StringVar(&description, "description", "", "Description")
	addServerFlag(cmd, &servers)
	cmd.Flags().BoolVar(&buildOnly, "build-only", false, "Only make the file available to build commands; never upload it to a server")
	cmd.MarkFlagsMutuallyExclusive("server", "build-only")
	return cmd
}

func newConfigFilesUpdateCmd() *cobra.Command {
	var path, body, description string
	var servers []string
	var allServers bool
	cmd := &cobra.Command{
		Use: "update <id>", Short: "Update a config file", Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			projectID, err := cliCtx.RequireProject()
			if err != nil {
				return err
			}
			client, err := cliCtx.Client()
			if err != nil {
				return err
			}
			// Only send fields the user explicitly set, so a partial update
			// (e.g. --description alone) doesn't clear unset path/body.
			var req sdk.ConfigFileUpdateRequest
			if cmd.Flags().Changed("path") {
				req.Path = &path
			}
			if cmd.Flags().Changed("body") {
				req.Body = &body
			}
			if cmd.Flags().Changed("description") {
				req.Description = &description
			}
			req.AllServers = allServers
			if cmd.Flags().Changed("server") {
				ids, err := resolveServerTargets(cliCtx.Background(), client, projectID, servers)
				if err != nil {
					return err
				}
				req.ServerIdentifiers = ids
			}
			f, err := client.UpdateConfigFile(cliCtx.Background(), projectID, args[0], req)
			if err != nil {
				return err
			}
			cliCtx.Envelope.Status("Updated config file: %s", f.Path)
			return nil
		},
	}
	cmd.Flags().StringVar(&path, "path", "", "File path")
	cmd.Flags().StringVar(&body, "body", "", "File content")
	cmd.Flags().StringVar(&description, "description", "", "Description")
	addServerFlag(cmd, &servers)
	addAllServersFlag(cmd, &allServers)
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
