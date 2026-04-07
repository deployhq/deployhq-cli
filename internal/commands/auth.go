package commands

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/deployhq/deployhq-cli/internal/auth"
	"github.com/deployhq/deployhq-cli/internal/config"
	"github.com/deployhq/deployhq-cli/internal/output"
	"github.com/deployhq/deployhq-cli/pkg/sdk"
	"github.com/manifoldco/promptui"
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

	// Update global config so all commands use the new account
	_ = config.Set(config.GlobalConfigPath(), "account", opts.Account)

	env.Status("Logged in as %s on %s.deployhq.com", opts.Email, opts.Account)
	return nil
}

func newAuthLogoutCmd() *cobra.Command {
	var account string
	var all bool
	cmd := &cobra.Command{
		Use:   "logout",
		Short: "Remove stored credentials",
		Long:  "Remove stored credentials. Shows a picker when multiple accounts are logged in.",
		RunE: func(cmd *cobra.Command, args []string) error {
			env := cliCtx.Envelope

			if all {
				auth.DeleteAll()
				env.Status("Logged out of all accounts")
				return nil
			}

			if account != "" {
				if err := auth.DeleteByAccount(account); err != nil {
					return &output.InternalError{Message: "remove credentials", Cause: err}
				}
				env.Status("Logged out of %s", account)
				return nil
			}

			// No --account flag — check how many profiles exist
			profiles := auth.ListProfiles()

			if len(profiles) == 0 {
				env.Status("Not logged in")
				return nil
			}

			if len(profiles) == 1 {
				if err := auth.DeleteByAccount(profiles[0].Account); err != nil {
					return &output.InternalError{Message: "remove credentials", Cause: err}
				}
				env.Status("Logged out of %s", profiles[0].Account)
				return nil
			}

			// Multiple accounts — show picker in TTY, error in non-TTY
			if !env.IsTTY {
				return &output.UserError{
					Message: "Multiple accounts logged in — specify which one",
					Hint:    "Use --account <name> or --all",
				}
			}

			items := make([]string, len(profiles))
			for i, p := range profiles {
				items[i] = fmt.Sprintf("%s (%s)", p.Account, p.Email)
			}
			prompt := promptui.Select{
				Label: "Select account to log out",
				Items: items,
			}
			idx, _, err := prompt.Run()
			if err != nil {
				return &output.UserError{Message: "Logout cancelled"}
			}
			selected := profiles[idx]
			if err := auth.DeleteByAccount(selected.Account); err != nil {
				return &output.InternalError{Message: "remove credentials", Cause: err}
			}
			env.Status("Logged out of %s", selected.Account)
			return nil
		},
	}
	cmd.Flags().StringVar(&account, "account", "", "Account profile to remove")
	cmd.Flags().BoolVar(&all, "all", false, "Log out of all accounts")
	return cmd
}

func newAuthStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show authentication status",
		RunE: func(cmd *cobra.Command, args []string) error {
			env := cliCtx.Envelope

			// Try configured account first, fall back to any stored credentials
			account := cliCtx.Config.Account
			creds, err := auth.LoadByAccount(account)
			if err != nil && account != "" {
				// Configured account not found — try default profile
				creds, err = auth.LoadByAccount("")
			}
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
			account := cliCtx.Config.Account
			creds, err := auth.LoadByAccount(account)
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
