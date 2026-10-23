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

// termsURL is DeployHQ's Terms of Service, which a person must accept to
// create an account.
const termsURL = "https://www.deployhq.com/terms"

// signupInOwnTerminalHint is shown when sign-up can't prompt (for example when
// run by an AI agent). The password must come from the person, typed at a
// hidden prompt, never relayed through (or invented by) an agent.
const signupInOwnTerminalHint = "Ask the person to run 'dhq signup' themselves in a separate terminal window " +
	"(not through the AI agent): it asks for the password at a hidden prompt and asks them to accept the " +
	"Terms of Service. Never ask for, or make up, a password in an AI chat."

func newSignupCmd() *cobra.Command {
	var email, password, accountName, fullName string
	var acceptTerms bool

	cmd := &cobra.Command{
		Use:   "signup",
		Short: "Create a new DeployHQ account",
		Long: `Sign up for a new DeployHQ account from your terminal. Credentials are saved automatically.

You are asked to accept DeployHQ's Terms of Service (` + termsURL + `). In
non-interactive mode, --accept-terms is required and states that the person
creating the account has read and accepted them.

After signing up, open the verification link DeployHQ emails you: until the
email address is verified, DeployHQ refuses API requests.

AI agents: do not run this with a password in the command. Ask the person to
run 'dhq signup' themselves in a separate terminal window.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			env := cliCtx.Envelope
			reader := bufio.NewReader(os.Stdin)

			// Interactive prompts for missing fields
			if email == "" {
				if env.NonInteractive {
					return &output.UserError{
						Message: "--email is required in non-interactive mode",
						Hint:    signupInOwnTerminalHint,
					}
				}
				fmt.Fprint(env.Stderr, "Email: ") //nolint:errcheck
				input, _ := reader.ReadString('\n')
				email = strings.TrimSpace(input)
			}
			if email == "" {
				return &output.UserError{Message: "Email is required"}
			}

			if password == "" {
				if env.NonInteractive {
					return &output.UserError{
						Message: "--password is required in non-interactive mode",
						Hint:    signupInOwnTerminalHint,
					}
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

			if accountName == "" && !env.NonInteractive {
				fmt.Fprint(env.Stderr, "Account name (optional, press Enter to auto-generate): ") //nolint:errcheck
				input, _ := reader.ReadString('\n')
				accountName = strings.TrimSpace(input)
			}

			if err := confirmSignupTerms(env, reader, acceptTerms); err != nil {
				return err
			}

			env.Status("Creating account...")

			ua := cliUserAgent()
			req := sdk.SignupRequest{
				Email:         email,
				Password:      password,
				AccountName:   accountName,
				FullName:      fullName,
				Client:        "dhq-cli",
				TermsAccepted: true, // confirmed above, by prompt or --accept-terms
			}

			result, err := sdk.Signup(req, ua, cliCtx.Config.SignupURL())
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

			if env.WantsJSON() {
				out := newSignupOutput(result, accountName, email)
				summary := fmt.Sprintf("Account created: %s", result.Account.Subdomain)
				if out.NextStep != "" {
					summary += ". " + out.NextStep
				}
				return env.WriteJSON(output.NewResponse(out, summary,
					output.Breadcrumb{Action: "check_status", Cmd: "dhq auth status"},
					output.Breadcrumb{Action: "projects", Cmd: "dhq projects list"},
				))
			}

			env.Status("")
			output.ColorGreen.Fprintf(env.Stderr, "Account created!\n") //nolint:errcheck
			env.Status("")
			host := "deployhq.com"
			if cliCtx.Config.Host != "" {
				host = cliCtx.Config.Host
			}
			env.Status("  Account:     %s.%s", result.Account.Subdomain, host)
			env.Status("  Email:       %s", email)
			env.Status("  SSH Key:     %s", result.SSHPublicKey.Fingerprint)
			env.Status("")
			reportSignupOutcome(env, accountName, result, email, host)
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
	cmd.Flags().BoolVar(&acceptTerms, "accept-terms", false, "Confirm the person has read and accepted the Terms of Service ("+termsURL+"); required in non-interactive mode")
	return cmd
}

// confirmSignupTerms makes sure the person accepted the Terms of Service before
// an account is created. Interactively it asks (an empty answer is a no); in
// non-interactive mode it requires --accept-terms rather than assuming consent.
func confirmSignupTerms(env *output.Envelope, reader *bufio.Reader, accepted bool) error {
	if accepted {
		return nil
	}
	if env.NonInteractive {
		return &output.UserError{
			Message: "Creating an account requires accepting DeployHQ's Terms of Service",
			Hint: "Re-run with --accept-terms only after the person has read and accepted " + termsURL + ". " +
				"Better: " + signupInOwnTerminalHint,
		}
	}
	env.Status("")
	env.Status("DeployHQ's Terms of Service: %s", termsURL)
	fmt.Fprint(env.Stderr, "Do you accept the Terms of Service? [y/N]: ") //nolint:errcheck
	answer, _ := reader.ReadString('\n')
	switch strings.ToLower(strings.TrimSpace(answer)) {
	case "y", "yes":
		return nil
	}
	return &output.UserError{
		Message: "The Terms of Service must be accepted to create an account",
		Hint:    "Review them at " + termsURL + ", then sign up again.",
	}
}

// signupOutput is the --json payload of 'dhq signup': the API response plus
// what the person needs to know about it.
type signupOutput struct {
	*sdk.SignupResponse
	// RequestedAccountName is set when the account name the person asked for
	// was not available as-is and a different subdomain was assigned.
	RequestedAccountName string `json:"requested_account_name,omitempty"`
	AccountNameChanged   bool   `json:"account_name_changed,omitempty"`
	NextStep             string `json:"next_step,omitempty"`
}

func newSignupOutput(result *sdk.SignupResponse, requestedName, email string) signupOutput {
	out := signupOutput{SignupResponse: result}
	if accountNameChanged(requestedName, result.Account.Subdomain) {
		out.RequestedAccountName = requestedName
		out.AccountNameChanged = true
	}
	if !result.EmailVerified {
		out.NextStep = verifyEmailNotice(email)
	}
	return out
}

// accountNameChanged reports whether the subdomain DeployHQ assigned differs
// from the account name the person asked for. Names are made unique (acme,
// acme2, acme3, ...) and normalised for use in a URL, so they can differ.
func accountNameChanged(requested, assigned string) bool {
	requested = strings.TrimSpace(requested)
	return requested != "" && !strings.EqualFold(requested, assigned)
}

func verifyEmailNotice(email string) string {
	return fmt.Sprintf("Check your inbox: DeployHQ sent a verification link to %s. "+
		"Open it before using the account; until then DeployHQ refuses API requests (deploys, project changes)", email)
}

// reportSignupOutcome tells the person, after a successful sign-up, about a
// changed account name and the email verification they still need to do.
func reportSignupOutcome(env *output.Envelope, requestedName string, result *sdk.SignupResponse, email, host string) {
	if accountNameChanged(requestedName, result.Account.Subdomain) {
		env.Warn("The account name %q wasn't available as-is, so your account address is %s.%s",
			requestedName, result.Account.Subdomain, host)
	}
	if !result.EmailVerified {
		env.Status("%s.", verifyEmailNotice(email))
		env.Status("")
	}
}
