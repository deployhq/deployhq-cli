package commands

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/deployhq/deployhq-cli/internal/output"
	"github.com/deployhq/deployhq-cli/pkg/sdk"
	"github.com/spf13/cobra"
)

func newBetaCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "beta",
		Short: "Manage DeployHQ beta enrollment",
		Long: `Enroll your account in DeployHQ's managed-resources beta (Static Hosting and
Managed VPS). Enrollment can only be flipped on by an account admin.`,
	}
	cmd.AddCommand(newBetaEnrollCmd())
	return cmd
}

func newBetaEnrollCmd() *cobra.Command {
	var protocol string
	cmd := &cobra.Command{
		Use:   "enroll",
		Short: "Enroll the account in the managed-resources beta",
		Long: `Enroll your account in the managed-resources beta.

By default enrolls in the managed_vps protocol; pass --protocol static_hosting to
enroll in Static Hosting instead. Requires an account admin — non-admins receive a
403 with a link to enable the beta in the dashboard.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := cliCtx.Client()
			if err != nil {
				return err
			}
			env := cliCtx.Envelope
			resp, err := client.EnrollBeta(cliCtx.Background(), protocol)
			if err != nil {
				var apiErr *sdk.APIError
				if errors.As(err, &apiErr) {
					betaURL := fmt.Sprintf("https://app.deployhq.com/%s/beta_features", client.Account())
					switch apiErr.StatusCode {
					case http.StatusForbidden:
						return &output.UserError{
							Message: "Beta enrollment requires an account admin. Ask an admin to enable it at: " + betaURL,
						}
					case http.StatusUnprocessableEntity:
						return &output.UserError{
							Message: fmt.Sprintf("Unsupported protocol %q. Use managed_vps or static_hosting.", protocol),
						}
					}
				}
				return err
			}
			if env.WantsJSON() {
				summary := "not enrolled"
				if resp.Enrolled {
					summary = "enrolled"
				}
				return env.WriteJSON(output.NewResponse(resp, summary))
			}
			if resp.Enrolled {
				output.ColorGreen.Fprintf(env.Stderr, "Enrolled in the managed-resources beta.\n") //nolint:errcheck
			} else {
				env.Status("Enrollment request sent, but the account is not yet enrolled.")
			}
			env.Status("\nTip: dhq launch to provision Static Hosting or a Managed VPS")
			return nil
		},
	}
	cmd.Flags().StringVar(&protocol, "protocol", "managed_vps", "Protocol to enroll in: managed_vps or static_hosting")
	return cmd
}
