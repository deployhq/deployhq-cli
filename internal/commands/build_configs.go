package commands

import (
	"fmt"

	"github.com/deployhq/deployhq-cli/internal/output"
	"github.com/spf13/cobra"
)

func newBuildConfigsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "build-configs",
		Short: "Manage build configurations",
	}
	cmd.AddCommand(
		&cobra.Command{
			Use: "list", Short: "List build configurations",
			RunE: func(cmd *cobra.Command, args []string) error {
				projectID, err := cliCtx.RequireProject()
				if err != nil {
					return err
				}
				client, err := cliCtx.Client()
				if err != nil {
					return err
				}
				configs, err := client.ListBuildConfigs(cliCtx.Background(), projectID)
				if err != nil {
					return err
				}
				env := cliCtx.Envelope
				if env.JSONMode || !env.IsTTY {
					return env.WriteJSON(output.NewResponse(configs, fmt.Sprintf("%d build configs", len(configs))))
				}
				rows := make([][]string, len(configs))
				for i, c := range configs {
					def := ""
					if c.Default {
						def = "(default)"
					}
					rows[i] = []string{c.Identifier, def}
				}
				env.WriteTable([]string{"Identifier", "Default"}, rows)
				return nil
			},
		},
		&cobra.Command{
			Use: "show <id>", Short: "Show a build configuration", Args: cobra.ExactArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				projectID, err := cliCtx.RequireProject()
				if err != nil {
					return err
				}
				client, err := cliCtx.Client()
				if err != nil {
					return err
				}
				config, err := client.GetBuildConfig(cliCtx.Background(), projectID, args[0])
				if err != nil {
					return err
				}
				return cliCtx.Envelope.WriteJSON(output.NewResponse(config, config.Identifier))
			},
		},
		&cobra.Command{
			Use: "default", Short: "Show default build configuration",
			RunE: func(cmd *cobra.Command, args []string) error {
				projectID, err := cliCtx.RequireProject()
				if err != nil {
					return err
				}
				client, err := cliCtx.Client()
				if err != nil {
					return err
				}
				config, err := client.GetDefaultBuildConfig(cliCtx.Background(), projectID)
				if err != nil {
					return err
				}
				return cliCtx.Envelope.WriteJSON(output.NewResponse(config, "Default build config"))
			},
		},
		&cobra.Command{
			Use: "delete <id>", Short: "Delete a build configuration", Args: cobra.ExactArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				projectID, err := cliCtx.RequireProject()
				if err != nil {
					return err
				}
				client, err := cliCtx.Client()
				if err != nil {
					return err
				}
				if err := client.DeleteBuildConfig(cliCtx.Background(), projectID, args[0]); err != nil {
					return err
				}
				cliCtx.Envelope.Status("Deleted build config: %s", args[0])
				return nil
			},
		},
	)
	return cmd
}
