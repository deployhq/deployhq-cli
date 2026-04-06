package commands

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/deployhq/deployhq-cli/internal/auth"
	"github.com/deployhq/deployhq-cli/internal/output"
	"github.com/deployhq/deployhq-cli/pkg/sdk"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

func newSignupCmd() *cobra.Command {
	var email, password, accountName, fullName string

	cmd := &cobra.Command{
		Use:   "signup",
		Short: "Create a new DeployHQ account",
		Long:  "Sign up for a new DeployHQ account from your terminal. Credentials are saved automatically.",
		RunE: func(cmd *cobra.Command, args []string) error {
			env := cliCtx.Envelope
			reader := bufio.NewReader(os.Stdin)

			// Interactive prompts for missing fields
			if email == "" {
				if !env.IsTTY {
					return &output.UserError{Message: "--email is required in non-interactive mode"}
				}
				fmt.Fprint(env.Stderr, "Email: ") //nolint:errcheck
				input, _ := reader.ReadString('\n')
				email = strings.TrimSpace(input)
			}
			if email == "" {
				return &output.UserError{Message: "Email is required"}
			}

			if password == "" {
				if !env.IsTTY {
					return &output.UserError{Message: "--password is required in non-interactive mode"}
				}
				fmt.Fprint(env.Stderr, "Password: ") //nolint:errcheck
				pw, err := term.ReadPassword(int(os.Stdin.Fd()))
				fmt.Fprintln(env.Stderr) //nolint:errcheck
				if err != nil {
					return &output.InternalError{Message: "read password", Cause: err}
				}
				password = strings.TrimSpace(string(pw))
			}
			if password == "" {
				return &output.UserError{Message: "Password is required"}
			}

			if accountName == "" && env.IsTTY {
				fmt.Fprint(env.Stderr, "Account name (optional, press Enter to auto-generate): ") //nolint:errcheck
				input, _ := reader.ReadString('\n')
				accountName = strings.TrimSpace(input)
			}

			env.Status("Creating account...")

			ua := cliUserAgent()
			req := sdk.SignupRequest{
				Email:       email,
				Password:    password,
				AccountName: accountName,
				FullName:    fullName,
				Client:      ua,
			}

			result, err := sdk.Signup(req, ua)
			if err != nil {
				return err
			}

			// Auto-save credentials
			if err := auth.Store(&auth.Credentials{
				Account: result.Account.Subdomain,
				Email:   email,
				APIKey:  result.APIKey,
			}); err != nil {
				env.Warn("Could not save credentials: %v", err)
				env.Status("API key: %s", result.APIKey)
			}

			if env.JSONMode || !env.IsTTY {
				return env.WriteJSON(output.NewResponse(result,
					fmt.Sprintf("Account created: %s", result.Account.Subdomain),
					output.Breadcrumb{Action: "login", Cmd: "dhq auth login"},
					output.Breadcrumb{Action: "projects", Cmd: "dhq projects list"},
				))
			}

			env.Status("")
			output.ColorGreen.Fprintf(env.Stderr, "Account created!\n") //nolint:errcheck
			env.Status("")
			env.Status("  Account:     %s.deployhq.com", result.Account.Subdomain)
			env.Status("  Email:       %s", email)
			env.Status("  SSH Key:     %s", result.SSHPublicKey.Fingerprint)
			env.Status("")
			env.Status("Credentials saved. Get started:")
			env.Status("  dhq projects list")
			env.Status("  dhq projects create --name my-app")
			return nil
		},
	}

	cmd.Flags().StringVar(&email, "email", "", "Email address")
	cmd.Flags().StringVar(&password, "password", "", "Password")
	cmd.Flags().StringVar(&accountName, "account-name", "", "Account subdomain (auto-generated if omitted)")
	cmd.Flags().StringVar(&fullName, "full-name", "", "Full name")
	return cmd
}
