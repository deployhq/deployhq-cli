package commands

import (
	"fmt"
	"strconv"

	"github.com/deployhq/deployhq-cli/internal/output"
	"github.com/deployhq/deployhq-cli/pkg/sdk"
	"github.com/spf13/cobra"
)

func newFoldersCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "folders",
		Short: "Manage project folders",
		Long: `Account-level folders (project categories) for organising projects into groups.

Folders are a purely organisational grouping — deleting a folder ungroups its projects, it does not delete them.`,
	}
	cmd.AddCommand(
		&cobra.Command{
			Use: "list", Short: "List folders",
			RunE: func(cmd *cobra.Command, args []string) error {
				client, err := cliCtx.Client()
				if err != nil {
					return err
				}
				folders, err := client.ListFolders(cliCtx.Background(), nil)
				if err != nil {
					return err
				}
				env := cliCtx.Envelope
				if env.WantsJSON() {
					return env.WriteJSON(output.NewResponse(folders, fmt.Sprintf("%d folders", len(folders)),
						output.Breadcrumb{Action: "create", Cmd: "dhq folders create <name>"},
						output.Breadcrumb{Action: "update", Cmd: "dhq folders update <identifier> --name <name>"},
						output.Breadcrumb{Action: "delete", Cmd: "dhq folders delete <identifier>"},
					))
				}
				if env.QuietMode {
					identifiers := make([]string, len(folders))
					for i, f := range folders {
						identifiers[i] = f.Identifier
					}
					env.WriteQuiet(identifiers)
					return nil
				}
				rows := make([][]string, len(folders))
				for i, f := range folders {
					position := ""
					if f.Position != nil {
						position = strconv.Itoa(*f.Position)
					}
					rows[i] = []string{f.Name, f.Identifier, strconv.Itoa(f.ProjectsCount), position}
				}
				env.WriteTable([]string{"Name", "Identifier", "Projects", "Position"}, rows)
				return nil
			},
		},
		&cobra.Command{
			Use: "create <name>", Short: "Create a folder", Args: cobra.ExactArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				client, err := cliCtx.Client()
				if err != nil {
					return err
				}
				f, err := client.CreateFolder(cliCtx.Background(), sdk.FolderCreateRequest{Name: args[0]})
				if err != nil {
					return err
				}
				env := cliCtx.Envelope
				if env.WantsJSON() {
					return env.WriteJSON(output.NewResponse(f, fmt.Sprintf("Created: %s", f.Name),
						output.Breadcrumb{Action: "update", Cmd: "dhq folders update <identifier> --name <name>", Resource: "folder", ID: f.Identifier},
						output.Breadcrumb{Action: "delete", Cmd: "dhq folders delete <identifier>", Resource: "folder", ID: f.Identifier},
					))
				}
				env.Status("Created folder: %s (%s)", f.Name, f.Identifier)
				return nil
			},
		},
		newFoldersUpdateCmd(),
		&cobra.Command{
			Use: "delete <identifier>", Short: "Delete a folder", Args: cobra.ExactArgs(1),
			Long: "Delete a folder. Projects in the folder are ungrouped, not deleted.",
			RunE: func(cmd *cobra.Command, args []string) error {
				client, err := cliCtx.Client()
				if err != nil {
					return err
				}
				if err := client.DeleteFolder(cliCtx.Background(), args[0]); err != nil {
					return err
				}
				env := cliCtx.Envelope
				if env.WantsJSON() {
					return env.WriteJSON(output.NewResponse(
						map[string]string{"identifier": args[0], "status": "deleted"},
						fmt.Sprintf("Deleted: %s", args[0]),
					))
				}
				env.Status("Deleted folder: %s", args[0])
				return nil
			},
		},
	)
	return cmd
}

func newFoldersUpdateCmd() *cobra.Command {
	var name string
	cmd := &cobra.Command{
		Use: "update <identifier>", Short: "Rename a folder", Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if name == "" {
				return &output.UserError{Message: "--name is required"}
			}
			client, err := cliCtx.Client()
			if err != nil {
				return err
			}
			f, err := client.UpdateFolder(cliCtx.Background(), args[0], sdk.FolderCreateRequest{Name: name})
			if err != nil {
				return err
			}
			env := cliCtx.Envelope
			if env.WantsJSON() {
				return env.WriteJSON(output.NewResponse(f, fmt.Sprintf("Updated: %s", f.Name),
					output.Breadcrumb{Action: "delete", Cmd: "dhq folders delete <identifier>", Resource: "folder", ID: f.Identifier},
					output.Breadcrumb{Action: "list", Cmd: "dhq folders list"},
				))
			}
			env.Status("Updated folder: %s (%s)", f.Name, f.Identifier)
			return nil
		},
	}
	cmd.Flags().StringVar(&name, "name", "", "New folder name (required)")
	return cmd
}
