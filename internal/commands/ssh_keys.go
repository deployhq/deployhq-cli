package commands

import (
	"fmt"
	"os"

	"github.com/deployhq/deployhq-cli/internal/output"
	"github.com/deployhq/deployhq-cli/pkg/sdk"
	"github.com/spf13/cobra"
)

func newSSHKeysCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "ssh-keys",
		Short: "Manage global SSH keys",
		Long: `Account-level SSH key pairs reusable across servers and projects. Generate or upload a key once, then reference it from any "dhq servers create --use-ssh-keys --global-key-pair-id <id>" call.

Centralizing keys here means rotation is one update instead of touching every server.`,
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
				if env.WantsJSON() {
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
		newSSHKeysDownloadCmd(),
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
			if env.WantsJSON() {
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

func newSSHKeysDownloadCmd() *cobra.Command {
	var outputFile string
	cmd := &cobra.Command{
		Use: "download <id>", Short: "Download an SSH key's private key", Args: cobra.ExactArgs(1),
		Long: `Download the private key material for a global SSH key.

Only available to account admins on a paid plan; other accounts receive a
permission error.

In an interactive terminal the raw key is printed to stdout. When output is
piped or redirected it is emitted as JSON (the standard CLI data contract), so
to save the raw key to a file use --output rather than shell redirection:

  dhq ssh-keys download <id> --output key.pem   # raw key, mode 0600

--output writes with secure owner-only (0600) permissions.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := cliCtx.Client()
			if err != nil {
				return err
			}
			privateKey, err := client.DownloadSSHKeyPrivateKey(cliCtx.Background(), args[0])
			if err != nil {
				return err
			}
			// Guard against a success response with no key material rather than
			// silently emitting an empty file / empty stdout.
			if privateKey == "" {
				return fmt.Errorf("server returned an empty private key for %s", args[0])
			}
			env := cliCtx.Envelope

			if outputFile != "" {
				if err := writeSecureFile(outputFile, privateKey); err != nil {
					return fmt.Errorf("write key to %s: %w", outputFile, err)
				}
				if env.WantsJSON() {
					return env.WriteJSON(output.NewResponse(
						map[string]string{"identifier": args[0], "output": outputFile, "status": "written"},
						fmt.Sprintf("Wrote private key to %s", outputFile),
					))
				}
				env.Status("Wrote private key to %s (mode 0600)", outputFile)
				return nil
			}

			if env.WantsJSON() {
				return env.WriteJSON(output.NewResponse(
					map[string]string{"identifier": args[0], "private_key": privateKey},
					"Private key downloaded",
				))
			}

			env.Warn("Printing private key material to stdout — handle with care.")
			fmt.Fprintln(env.Stdout, privateKey)
			return nil
		},
	}
	cmd.Flags().StringVarP(&outputFile, "output", "o", "", "Write the private key to a file (mode 0600) instead of stdout")
	return cmd
}

// writeSecureFile writes private key material with owner-only (0600)
// permissions. Unlike os.WriteFile, it also tightens the mode of a
// pre-existing file — os.WriteFile leaves an existing file's permissions
// untouched, which could leave key material world-readable.
func writeSecureFile(path, contents string) error {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	// Enforce 0600 even if the file already existed with looser permissions.
	if err := f.Chmod(0o600); err != nil {
		_ = f.Close()
		return err
	}
	if _, err := f.WriteString(contents); err != nil {
		_ = f.Close()
		return err
	}
	return f.Close()
}
