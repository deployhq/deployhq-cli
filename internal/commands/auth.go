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

func newAuthCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "auth",
		Short: "Manage authentication",
		Long:  "Login, logout, and check authentication status.",
	}

	cmd.AddCommand(
		newAuthLoginCmd(),
		newAuthLogoutCmd(),
		newAuthStatusCmd(),
		newAuthTokenCmd(),
	)

	return cmd
}

// AuthLoginOptions holds the options for auth login (for testing via runF injection).
type AuthLoginOptions struct {
	Account string
	Email   string
	APIKey  string
	RunF    func(*AuthLoginOptions) error
}

func newAuthLoginCmd() *cobra.Command {
	opts := &AuthLoginOptions{}

	cmd := &cobra.Command{
		Use:   "login",
		Short: "Authenticate with DeployHQ",
		Long:  "Login with your DeployHQ account credentials. Provide --account, --email, and --api-key flags, or enter them interactively.",
		RunE: func(cmd *cobra.Command, args []string) error {
			if opts.RunF != nil {
				return opts.RunF(opts)
			}
			return runAuthLogin(opts)
		},
	}

	cmd.Flags().StringVar(&opts.Account, "account", "", "Account subdomain (e.g. 'mycompany' for mycompany.deployhq.com)")
	cmd.Flags().StringVar(&opts.Email, "email", "", "Login email address")
	cmd.Flags().StringVar(&opts.APIKey, "api-key", "", "API key (from Profile > API Key in DeployHQ)")

	return cmd
}

func runAuthLogin(opts *AuthLoginOptions) error {
	env := cliCtx.Envelope
	reader := bufio.NewReader(os.Stdin)

	// Interactive prompts for missing values (two-tier: flag > prompt)
	if opts.Account == "" {
		if !env.IsTTY {
			return &output.UserError{
				Message: "Account is required in non-interactive mode",
				Hint:    "Use --account flag",
			}
		}
		fmt.Fprint(env.Stderr, "Account subdomain: ") //nolint:errcheck // best-effort stderr
		input, _ := reader.ReadString('\n')
		opts.Account = strings.TrimSpace(input)
	}

	if opts.Email == "" {
		if !env.IsTTY {
			return &output.UserError{
				Message: "Email is required in non-interactive mode",
				Hint:    "Use --email flag",
			}
		}
		fmt.Fprint(env.Stderr, "Email: ") //nolint:errcheck // best-effort stderr
		input, _ := reader.ReadString('\n')
		opts.Email = strings.TrimSpace(input)
	}

	if opts.APIKey == "" {
		if !env.IsTTY {
			return &output.UserError{
				Message: "API key is required in non-interactive mode",
				Hint:    "Use --api-key flag",
			}
		}
		fmt.Fprint(env.Stderr, "API key: ") //nolint:errcheck // best-effort stderr
		key, err := term.ReadPassword(int(os.Stdin.Fd()))
		if err == nil && len(key) > 0 {
			masked := strings.Repeat("*", len(key))
			fmt.Fprintf(env.Stderr, "%s\n", masked) //nolint:errcheck // best-effort stderr
		} else {
			fmt.Fprintln(env.Stderr) //nolint:errcheck // best-effort stderr
		}
		if err != nil {
			return &output.InternalError{Message: "read api key", Cause: err}
		}
		opts.APIKey = strings.TrimSpace(string(key))
	}

	// Validate by making a test API call
	env.Status("Validating credentials...")
	client, err := sdk.New(opts.Account, opts.Email, opts.APIKey)
	if err != nil {
		return &output.UserError{Message: err.Error()}
	}

	_, err = client.ListProjects(cliCtx.Background())
	if err != nil {
		if sdk.IsUnauthorized(err) {
			return &output.AuthError{
				Message: "Invalid credentials",
				Hint:    "Check your email and API key at Profile > API Key in DeployHQ",
			}
		}
		return &output.InternalError{Message: "validate credentials", Cause: err}
	}

	// Store credentials
	creds := &auth.Credentials{
		Account: opts.Account,
		Email:   opts.Email,
		APIKey:  opts.APIKey,
	}
	if err := auth.Store(creds); err != nil {
		return &output.InternalError{Message: "store credentials", Cause: err}
	}

	env.Status("Logged in as %s on %s.deployhq.com", opts.Email, opts.Account)
	return nil
}

func newAuthLogoutCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "logout",
		Short: "Remove stored credentials",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := auth.Delete(); err != nil {
				return &output.InternalError{Message: "remove credentials", Cause: err}
			}
			cliCtx.Envelope.Status("Logged out")
			return nil
		},
	}
}

func newAuthStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show authentication status",
		RunE: func(cmd *cobra.Command, args []string) error {
			env := cliCtx.Envelope

			creds, err := auth.Load()
			if err != nil {
				env.Status("Not logged in")
				return nil
			}

			if cliCtx.Envelope.JSONMode {
				return env.WriteJSON(map[string]string{
					"account": creds.Account,
					"email":   creds.Email,
					"status":  "authenticated",
				})
			}

			env.Status("Account: %s.deployhq.com", creds.Account)
			env.Status("Email:   %s", creds.Email)
			env.Status("Status:  authenticated")
			return nil
		},
	}
}

func newAuthTokenCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "token",
		Short: "Print the stored API key to stdout",
		Long:  "Print the API key to stdout for use in scripts and pipelines.",
		RunE: func(cmd *cobra.Command, args []string) error {
			creds, err := auth.Load()
			if err != nil {
				return &output.AuthError{
					Message: "Not logged in",
					Hint:    "Run 'dhq auth login' first",
				}
			}
			fmt.Fprintln(cliCtx.Envelope.Stdout, creds.APIKey) //nolint:errcheck
			return nil
		},
	}
}
