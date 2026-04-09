package commands

import (
	"fmt"

	"github.com/deployhq/deployhq-cli/internal/output"
	"github.com/deployhq/deployhq-cli/pkg/sdk"
	"github.com/spf13/cobra"
)

func newSSHKeysCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "ssh-keys",
		Short: "Manage global SSH keys",
	}
	cmd.AddCommand(
		&cobra.Command{
			Use: "list", Short: "List SSH keys",
			RunE: func(cmd *cobra.Command, args []string) error {
				client, err := cliCtx.Client()
				if err != nil {
					return err
				}
				keys, err := client.ListSSHKeys(cliCtx.Background(), nil)
				if err != nil {
					return err
				}
				env := cliCtx.Envelope
				if env.JSONMode || !env.IsTTY {
					return env.WriteJSON(output.NewResponse(keys, fmt.Sprintf("%d SSH keys", len(keys))))
				}
				rows := make([][]string, len(keys))
				for i, k := range keys {
					rows[i] = []string{k.Title, k.Identifier, k.KeyType, k.Fingerprint}
				}
				env.WriteTable([]string{"Title", "Identifier", "Type", "Fingerprint"}, rows)
				return nil
			},
		},
		newSSHKeysCreateCmd(),
		&cobra.Command{
			Use: "delete <id>", Short: "Delete an SSH key", Args: cobra.ExactArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				client, err := cliCtx.Client()
				if err != nil {
					return err
				}
				if err := client.DeleteSSHKey(cliCtx.Background(), args[0]); err != nil {
					return err
				}
				cliCtx.Envelope.Status("Deleted SSH key: %s", args[0])
				return nil
			},
		},
	)
	return cmd
}

func newSSHKeysCreateCmd() *cobra.Command {
	var title, keyType string
	cmd := &cobra.Command{
		Use: "create", Short: "Create an SSH key (generated server-side)",
		RunE: func(cmd *cobra.Command, args []string) error {
			if title == "" {
				return &output.UserError{Message: "--title is required"}
			}
			client, err := cliCtx.Client()
			if err != nil {
				return err
			}
			k, err := client.CreateSSHKey(cliCtx.Background(), sdk.SSHKeyCreateRequest{
				Title: title, KeyType: keyType,
			})
			if err != nil {
				return err
			}
			env := cliCtx.Envelope
			if env.JSONMode || !env.IsTTY {
				return env.WriteJSON(output.NewResponse(k, fmt.Sprintf("Created: %s", k.Title)))
			}
			env.Status("Created SSH key: %s (%s)", k.Title, k.KeyType)
			env.Status("Fingerprint: %s", k.Fingerprint)
			return nil
		},
	}
	cmd.Flags().StringVar(&title, "title", "", "Key title (required)")
	cmd.Flags().StringVar(&keyType, "type", "ED25519", "Key type: RSA or ED25519")
	return cmd
}
