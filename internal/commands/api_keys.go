package commands

import (
	"fmt"

	"github.com/deployhq/deployhq-cli/internal/output"
	"github.com/deployhq/deployhq-cli/pkg/sdk"
	"github.com/spf13/cobra"
)

func newAPIKeysCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "api-keys",
		Short: "Manage API keys",
		Long: `Account API keys.

The plaintext key is shown ONCE, at creation time. Store it securely — it cannot be retrieved again.`,
	}
	cmd.AddCommand(
		newAPIKeysCreateCmd(),
		newAPIKeysDeleteCmd(),
	)
	return cmd
}

func newAPIKeysCreateCmd() *cobra.Command {
	var (
		description string
		readOnly    bool
	)
	cmd := &cobra.Command{
		Use: "create", Short: "Create an API key",
		RunE: func(cmd *cobra.Command, args []string) error {
			if description == "" {
				return &output.UserError{Message: "--description is required"}
			}
			client, err := cliCtx.Client()
			if err != nil {
				return err
			}
			req := sdk.APIKeyCreateRequest{Description: description}
			if cmd.Flags().Changed("read-only") {
				req.ReadOnly = &readOnly
			}
			k, err := client.CreateAPIKey(cliCtx.Background(), req)
			if err != nil {
				return err
			}
			env := cliCtx.Envelope
			if env.WantsJSON() {
				// In JSON mode the plaintext key is included in the payload as-is.
				return env.WriteJSON(output.NewResponse(k, fmt.Sprintf("Created API key: %s", k.Description),
					output.Breadcrumb{Action: "delete", Cmd: "dhq api-keys delete <identifier>", Resource: "api_key", ID: k.Identifier},
				))
			}
			// Human output: surface the plaintext key prominently with a warning
			// that this is the only time it will ever be shown.
			env.Status("Created API key: %s (%s)", k.Description, k.Identifier)
			env.Status("")
			env.Status("  API key: %s", k.Key)
			env.Status("")
			env.Warn("This is the ONLY time the key will be shown. Store it securely now.")
			return nil
		},
	}
	cmd.Flags().StringVar(&description, "description", "", "Key description (required)")
	cmd.Flags().BoolVar(&readOnly, "read-only", false, "Create a read-only key")
	return cmd
}

func newAPIKeysDeleteCmd() *cobra.Command {
	return &cobra.Command{
		Use: "delete <identifier>", Short: "Delete an API key", Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := cliCtx.Client()
			if err != nil {
				return err
			}
			if err := client.DeleteAPIKey(cliCtx.Background(), args[0]); err != nil {
				return err
			}
			env := cliCtx.Envelope
			if env.WantsJSON() {
				return env.WriteJSON(output.NewResponse(
					map[string]string{"identifier": args[0], "status": "deleted"},
					fmt.Sprintf("Deleted: %s", args[0]),
				))
			}
			env.Status("Deleted API key: %s", args[0])
			return nil
		},
	}
}
