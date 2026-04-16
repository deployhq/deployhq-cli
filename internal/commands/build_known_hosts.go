package commands

import (
	"fmt"

	"github.com/deployhq/deployhq-cli/internal/output"
	"github.com/deployhq/deployhq-cli/pkg/sdk"
	"github.com/spf13/cobra"
)

func newBuildKnownHostsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "build-known-hosts",
		Short: "Manage build server SSH known hosts",
	}
	cmd.AddCommand(
		&cobra.Command{
			Use: "list", Short: "List build known hosts",
			RunE: func(cmd *cobra.Command, args []string) error {
				projectID, err := cliCtx.RequireProject()
				if err != nil {
					return err
				}
				client, err := cliCtx.Client()
				if err != nil {
					return err
				}
				hosts, err := client.ListBuildKnownHosts(cliCtx.Background(), projectID)
				if err != nil {
					return err
				}
				env := cliCtx.Envelope
				if env.JSONMode || !env.IsTTY {
					return env.WriteJSON(output.NewResponse(hosts, fmt.Sprintf("%d build known hosts", len(hosts))))
				}
				rows := make([][]string, len(hosts))
				for i, h := range hosts {
					rows[i] = []string{h.Identifier, h.Hostname}
				}
				env.WriteTable([]string{"Identifier", "Hostname"}, rows)
				return nil
			},
		},
		newBuildKnownHostsCreateCmd(),
		&cobra.Command{
			Use: "delete <id>", Short: "Delete a build known host", Args: cobra.ExactArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				projectID, err := cliCtx.RequireProject()
				if err != nil {
					return err
				}
				client, err := cliCtx.Client()
				if err != nil {
					return err
				}
				if err := client.DeleteBuildKnownHost(cliCtx.Background(), projectID, args[0]); err != nil {
					return err
				}
				cliCtx.Envelope.Status("Deleted build known host: %s", args[0])
				return nil
			},
		},
	)
	return cmd
}

func newBuildKnownHostsCreateCmd() *cobra.Command {
	var hostname, publicKey string
	cmd := &cobra.Command{
		Use: "create", Short: "Create a build known host",
		RunE: func(cmd *cobra.Command, args []string) error {
			if hostname == "" || publicKey == "" {
				return &output.UserError{Message: "Both --hostname and --public-key are required"}
			}
			projectID, err := cliCtx.RequireProject()
			if err != nil {
				return err
			}
			client, err := cliCtx.Client()
			if err != nil {
				return err
			}
			h, err := client.CreateBuildKnownHost(cliCtx.Background(), projectID, sdk.BuildKnownHostCreateRequest{
				Hostname: hostname, PublicKey: publicKey,
			})
			if err != nil {
				return err
			}
			env := cliCtx.Envelope
			if env.JSONMode || !env.IsTTY {
				return env.WriteJSON(output.NewResponse(h, fmt.Sprintf("Created: %s", h.Hostname)))
			}
			env.Status("Created build known host: %s", h.Hostname)
			return nil
		},
	}
	cmd.Flags().StringVar(&hostname, "hostname", "", "Hostname (required)")
	cmd.Flags().StringVar(&publicKey, "public-key", "", "SSH public key (required)")
	return cmd
}
