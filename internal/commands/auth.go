package commands

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/deployhq/deployhq-cli/internal/auth"
	"github.com/deployhq/deployhq-cli/internal/cli"
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
		Long: `Login with your DeployHQ account credentials. Run it in your own terminal: it
prompts for the account, email and API key (the key is typed hidden) and stores
them in the OS keychain.

The API key is under Settings → Security → API Credentials in DeployHQ
(https://<your-account>.deployhq.com/security).

AI agents: do not ask the person for their API key. Ask them to run
'dhq auth login' themselves in a separate terminal window, never to paste the
key into the chat. For CI, set DEPLOYHQ_ACCOUNT, DEPLOYHQ_EMAIL and
DEPLOYHQ_API_KEY as secret environment variables instead of logging in.`,
		Example: `  # Interactive login (prompts for any missing values)
  dhq auth login

  # Pre-fill the account and email; the API key is still prompted for (hidden)
  dhq auth login --account mycompany --email me@example.com`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if opts.RunF != nil {
				return opts.RunF(opts)
			}
			return runAuthLogin(opts)
		},
	}

	cmd.Flags().StringVar(&opts.Account, "account", "", "Account subdomain (e.g. 'mycompany' for mycompany.deployhq.com)")
	cmd.Flags().StringVar(&opts.Email, "email", "", "Login email address")
	cmd.Flags().StringVar(&opts.APIKey, "api-key", "", "API key (Settings → Security → API Credentials in DeployHQ). Prefer the hidden prompt; never paste a key into an AI chat")

	return cmd
}

// normalizeAccount accepts user-supplied account values in any of these forms
// and returns just the subdomain:
//
//	mycompany
//	mycompany.deployhq.com
//	https://mycompany.deployhq.com/
//
// host is the configured DeployHQ host (defaults to "deployhq.com") so the
// same logic works for staging/self-hosted setups.
func normalizeAccount(input, host string) string {
	return cli.NormalizeAccount(input, host)
}

func runAuthLogin(opts *AuthLoginOptions) error {
	env := cliCtx.Envelope
	reader := bufio.NewReader(os.Stdin)

	// Non-interactive (AI agents, pipes, CI): dhq can't prompt, and the API key
	// must not be relayed through an agent. Point the person at signing in
	// themselves rather than at passing the key as a flag.
	if env.NonInteractive && (opts.Account == "" || opts.Email == "" || opts.APIKey == "") {
		return &output.AuthError{
			Message: "Signing in needs your DeployHQ API key, which dhq can't ask for in non-interactive mode",
			Hint:    auth.SignInHint(normalizeAccount(opts.Account, cliCtx.Config.Host), cliCtx.Config.Host),
		}
	}

	// Interactive prompts for missing values (two-tier: flag > prompt)
	if opts.Account == "" {
		fmt.Fprint(env.Stderr, "Account subdomain (e.g. 'mycompany' for mycompany.deployhq.com): ") //nolint:errcheck // best-effort stderr
		input, _ := reader.ReadString('\n')
		opts.Account = strings.TrimSpace(input)
	}
	opts.Account = normalizeAccount(opts.Account, cliCtx.Config.Host)

	if opts.Email == "" {
		fmt.Fprint(env.Stderr, "Email: ") //nolint:errcheck // best-effort stderr
		input, _ := reader.ReadString('\n')
		opts.Email = strings.TrimSpace(input)
	}

	if opts.APIKey == "" {
		env.Status("Your API key is under %s.", auth.APIKeyLocation(opts.Account, cliCtx.Config.Host))
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
	var sdkOpts []sdk.Option
	if baseURL := cliCtx.Config.BaseURL(opts.Account); baseURL != "" {
		sdkOpts = append(sdkOpts, sdk.WithBaseURL(baseURL))
	}
	client, err := sdk.New(opts.Account, opts.Email, opts.APIKey, sdkOpts...)
	if err != nil {
		return &output.UserError{Message: err.Error()}
	}

	_, err = client.ListProjects(cliCtx.Background(), nil)
	emailUnverified := output.IsEmailVerificationRequired(err)
	if err != nil && !emailUnverified {
		// The credentials are fine but the account's plan or billing state
		// refuses API access: say so rather than "access denied".
		var acctErr *output.AccountError
		if errors.As(output.ExplainAccountError(err), &acctErr) {
			return acctErr
		}
		if sdk.IsUnauthorized(err) {
			return &output.AuthError{
				Message: "Invalid credentials",
				Hint:    "Check your email and API key. The API key is under " + auth.APIKeyLocation(opts.Account, cliCtx.Config.Host) + ".",
			}
		}
		if sdk.IsForbidden(err) {
			return &output.AuthError{
				Message: "Access denied",
				Hint:    "Your account may have API access restricted, or your user may not be a member of this account. Check Account Settings > API.",
			}
		}
		if output.IsNetworkErr(err) {
			return &output.NetworkError{Message: "validate credentials", Cause: err}
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

	host := "deployhq.com"
	if cliCtx.Config.Host != "" {
		host = cliCtx.Config.Host
	}
	env.Status("Logged in as %s on %s.%s", opts.Email, opts.Account, host)
	// The credentials are valid, but the API refuses every request until the
	// account's email is verified: say so now rather than on the next command.
	if emailUnverified {
		env.Warn("%s.", output.EmailNotVerifiedMessage)
		env.Status("%s", output.EmailNotVerifiedHint)
	}
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

			// Multiple accounts — show picker or fail in non-interactive
			if env.NonInteractive {
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

// Values of the auth status "status" field.
const (
	authStatusAuthenticated     = "authenticated"
	authStatusEmailUnverified   = "email_verification_required"
	authStatusInvalidCredential = "invalid_credentials"
	authStatusAPIRestricted     = "api_access_restricted"
	authStatusUnconfirmed       = "unconfirmed"
)

// authStatusResult is the --json shape of 'dhq auth status'.
type authStatusResult struct {
	Account string `json:"account"`
	Email   string `json:"email"`
	// Source says where the credentials came from: "env", "flag", "file",
	// "stored" (a login saved by 'dhq auth login'), or "mixed".
	Source string `json:"source"`
	// Sources gives the source of each of account, email and api_key.
	Sources  map[string]string `json:"sources,omitempty"`
	Status   string            `json:"status"`
	Message  string            `json:"message,omitempty"`
	NextStep string            `json:"next_step,omitempty"`
}

// credentialSource summarises per-field sources into one value: the shared
// source when all fields agree, otherwise "mixed".
func credentialSource(sources map[string]string) string {
	src := ""
	for _, key := range []string{"account", "email", "api_key"} {
		v := sources[key]
		if v == "" {
			v = "config"
		}
		if src == "" {
			src = v
		} else if src != v {
			return "mixed"
		}
	}
	return src
}

// describeCredentialSource is the human form of credentialSource.
func describeCredentialSource(sources map[string]string) string {
	names := map[string]string{
		"env":                      "environment variables (DEPLOYHQ_*)",
		"flag":                     "command-line flags",
		"file":                     "config file",
		cli.CredentialSourceStored: "stored login ('dhq auth login')",
		"config":                   "configuration",
	}
	if src := credentialSource(sources); src != "mixed" {
		return names[src]
	}
	parts := make([]string, 0, 3)
	for _, key := range []string{"account", "email", "api_key"} {
		v := sources[key]
		if v == "" {
			v = "config"
		}
		parts = append(parts, fmt.Sprintf("%s from %s", key, names[v]))
	}
	return strings.Join(parts, ", ")
}

// checkAuthStatus asks the API whether the credentials dhq would use actually
// work. Having credentials is not enough: the API refuses every request until
// the account's email is verified, and a key may have been revoked.
func checkAuthStatus(ctx context.Context, client *sdk.Client, creds *cli.ResolvedCredentials, host string) authStatusResult {
	res := authStatusResult{
		Account: creds.Account, Email: creds.Email,
		Source: credentialSource(creds.Sources), Sources: creds.Sources,
	}
	// Same /profile request launch uses; it decodes only a few fields, so it
	// does not depend on the full profile shape.
	_, err := client.GetAccountCapabilities(ctx)
	var apiErr *sdk.APIError
	isAPIErr := errors.As(err, &apiErr)
	switch {
	case err == nil:
		res.Status = authStatusAuthenticated
	case output.IsEmailVerificationRequired(err):
		res.Status = authStatusEmailUnverified
		res.Message = output.EmailNotVerifiedMessage
		res.NextStep = output.EmailNotVerifiedHint
	case isAPIErr && apiErr.StatusCode == http.StatusForbidden && apiErr.Message == "api_access_restricted":
		// The key is valid, but the account's plan does not include API access.
		res.Status = authStatusAPIRestricted
		res.Message = "DeployHQ accepted the credentials, but this account's plan does not include API access"
		h := host
		if h == "" {
			h = "deployhq.com"
		}
		res.NextStep = fmt.Sprintf("Upgrade the account's plan (https://%s.%s/account/packages), then re-run 'dhq auth status'.", creds.Account, h)
	case isAPIErr && (apiErr.StatusCode == http.StatusUnauthorized || apiErr.StatusCode == http.StatusForbidden):
		// A wrong or revoked API key gets a 403 AccessDenied (401 on some paths).
		res.Status = authStatusInvalidCredential
		res.Message = fmt.Sprintf("DeployHQ rejected the API key for account %q (wrong, or revoked)", creds.Account)
		res.NextStep = invalidCredentialsNextStep(creds, host)
	case isAPIErr && apiErr.StatusCode == http.StatusNotFound:
		// An unknown account subdomain answers 404.
		res.Status = authStatusInvalidCredential
		res.Message = fmt.Sprintf("DeployHQ has no account %q", creds.Account)
		res.NextStep = invalidCredentialsNextStep(creds, host)
	case output.IsNetworkErr(err) || (isAPIErr && apiErr.StatusCode >= 500):
		res.Status = authStatusUnconfirmed
		res.Message = fmt.Sprintf("The credentials could not be checked with DeployHQ: %v", err)
		res.NextStep = "Check your connection and re-run 'dhq auth status'."
	default:
		res.Status = authStatusUnconfirmed
		res.Message = fmt.Sprintf("The credentials could not be checked with DeployHQ: %v", err)
		res.NextStep = "Re-run 'dhq auth status'."
	}
	return res
}

// invalidCredentialsNextStep says how to fix rejected credentials, which
// depends on where they came from: a stored login is replaced by signing in
// again; configured values have to be corrected where they are set.
func invalidCredentialsNextStep(creds *cli.ResolvedCredentials, host string) string {
	keyAt := "The API key is under " + auth.APIKeyLocation(creds.Account, host) + "."
	switch creds.Sources["api_key"] {
	case "env":
		return "Check DEPLOYHQ_ACCOUNT and DEPLOYHQ_API_KEY in the environment. " + keyAt
	case "flag":
		return "Check the --account and --api-key values. " + keyAt
	case "file":
		return "Check the account and api_key in the dhq config file ('dhq configure show'). " + keyAt
	}
	return "Ask the person to sign in again by running 'dhq auth login' in a separate terminal window. " + keyAt
}

func newAuthStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show authentication status",
		Long: `Show which account dhq is signed in to, and check with DeployHQ that the
credentials work. Credentials are resolved exactly as for API commands: flags,
DEPLOYHQ_* environment variables and config files win, and the stored login
fills any gaps. The output says where they came from.

The status is one of:
  authenticated                 credentials work
  email_verification_required   signed in, but DeployHQ refuses requests until the
                                account's email address is verified
  invalid_credentials           DeployHQ rejected the API key, or has no such account
  api_access_restricted         the credentials work, but the plan has no API access
  unconfirmed                   DeployHQ could not be reached to check`,
		RunE: func(cmd *cobra.Command, args []string) error {
			env := cliCtx.Envelope

			// Resolve credentials exactly as API commands do, so status checks
			// and reports the account those commands would use.
			creds, err := cliCtx.ResolveCredentials()
			if err != nil {
				env.Status("Not logged in")
				return nil
			}

			client, err := cliCtx.NewClient(creds.Account, creds.Email, creds.APIKey)
			if err != nil {
				return &output.UserError{Message: err.Error()}
			}
			res := checkAuthStatus(cliCtx.Background(), client, creds, cliCtx.Config.Host)

			if cliCtx.Envelope.JSONMode {
				return env.WriteJSON(res)
			}

			host := "deployhq.com"
			if cliCtx.Config.Host != "" {
				host = cliCtx.Config.Host
			}
			env.Status("Account: %s.%s", creds.Account, host)
			env.Status("Email:   %s", creds.Email)
			env.Status("Source:  %s", describeCredentialSource(creds.Sources))
			env.Status("Status:  %s", res.Status)
			if res.Message != "" {
				env.Status("")
				env.Status("%s.", strings.TrimSuffix(res.Message, "."))
			}
			if res.NextStep != "" {
				env.Status("Next: %s", res.NextStep)
			}
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
