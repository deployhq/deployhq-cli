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

func newHelloCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "hello",
		Short: "Get started with DeployHQ CLI",
		Long:  "Guided setup that walks you through login, account creation, and project configuration.",
		RunE: func(cmd *cobra.Command, args []string) error {
			env := cliCtx.Envelope

			if !env.IsTTY {
				return &output.UserError{
					Message: "Interactive setup requires a terminal",
					Hint:    "Use 'dhq auth login' and 'dhq configure' for non-interactive setup",
				}
			}

			env.Status("")
			output.ColorGreen.Fprintf(env.Stderr, "Welcome to DeployHQ CLI!\n") //nolint:errcheck
			env.Status("")

			// Step 1: Authentication
			loggedIn, creds := checkAuth()
			if !loggedIn {
				var err error
				creds, err = helloAuth(env)
				if err != nil {
					return err
				}
			} else {
				env.Status("Logged in as %s on %s.deployhq.com", creds.Email, creds.Account)
			}

			// Step 2: Fetch projects
			client, err := sdk.New(creds.Account, creds.Email, creds.APIKey)
			if err != nil {
				return &output.InternalError{Message: "create client", Cause: err}
			}

			projects, err := client.ListProjects(cliCtx.Background())
			if err != nil {
				env.Warn("Could not fetch projects: %v", err)
				env.Status("")
				env.Status("Get started:")
				env.Status("  dhq init              Create a project with repo and server")
				env.Status("  dhq projects create   Create a project")
				return nil
			}

			if len(projects) == 0 {
				env.Status("")
				env.Status("No projects yet.")

				prompt := promptui.Select{
					Label: "Create your first project?",
					Items: []string{"Yes, run guided setup (dhq init)", "No, I'll do it later"},
				}

				idx, _, err := prompt.Run()
				if err != nil || idx == 1 {
					env.Status("")
					env.Status("When you're ready:")
					env.Status("  dhq init              Guided project setup (repo + server + deploy)")
					env.Status("  dhq projects create   Create a project manually")
					return nil
				}

				// Run dhq init
				initCmd := newInitCmd()
				return initCmd.RunE(initCmd, nil)
			}

			// Step 3: Default project
			defaultProject := cliCtx.Config.Project
			if defaultProject != "" {
				env.Status("Default project: %s", defaultProject)
			} else {
				env.Status("")
				env.Status("You have %d project(s). Pick a default for this directory:", len(projects))

				items := make([]string, len(projects))
				for i, p := range projects {
					items[i] = fmt.Sprintf("%s (%s)", p.Name, p.Permalink)
				}

				prompt := promptui.Select{
					Label: "Default project",
					Items: items,
				}

				idx, _, err := prompt.Run()
				if err != nil {
					return &output.UserError{Message: "Setup cancelled"}
				}

				project := projects[idx]
				path := config.ProjectConfigPath()
				if err := config.Set(path, "project", project.Permalink); err != nil {
					return &output.InternalError{Message: "save config", Cause: err}
				}

				defaultProject = project.Permalink
				env.Status("")
				output.ColorGreen.Fprintf(env.Stderr, "Saved to %s\n", path) //nolint:errcheck
			}

			// Step 4: Orientation
			env.Status("")
			env.Status("You're all set! Here are some useful commands:")
			env.Status("  dhq deploy            Deploy your project")
			env.Status("  dhq deployments list  View deployment history")
			env.Status("  dhq servers list      View servers")
			env.Status("  dhq status            Dashboard overview")
			env.Status("  dhq open              Open project in browser")
			return nil
		},
	}
}

func checkAuth() (bool, *auth.Credentials) {
	creds, err := auth.LoadByAccount(cliCtx.Config.Account)
	if err != nil || creds.APIKey == "" {
		return false, nil
	}
	return true, creds
}

func helloAuth(env *output.Envelope) (*auth.Credentials, error) {
	reader := bufio.NewReader(os.Stdin)

	// Ask if they have an account
	prompt := promptui.Select{
		Label: "Do you have a DeployHQ account?",
		Items: []string{"Yes, log me in", "No, create one"},
	}

	idx, _, err := prompt.Run()
	if err != nil {
		return nil, &output.UserError{Message: "Setup cancelled"}
	}

	if idx == 0 {
		return helloLogin(env, reader)
	}
	return helloSignup(env, reader)
}

func helloLogin(env *output.Envelope, reader *bufio.Reader) (*auth.Credentials, error) {
	env.Status("")

	fmt.Fprint(env.Stderr, "Account subdomain: ") //nolint:errcheck
	account, _ := reader.ReadString('\n')
	account = strings.TrimSpace(account)

	fmt.Fprint(env.Stderr, "Email: ") //nolint:errcheck
	email, _ := reader.ReadString('\n')
	email = strings.TrimSpace(email)

	fmt.Fprint(env.Stderr, "API key: ") //nolint:errcheck
	key, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Fprintln(env.Stderr) //nolint:errcheck
	if err != nil {
		return nil, &output.InternalError{Message: "read API key", Cause: err}
	}
	apiKey := strings.TrimSpace(string(key))

	env.Status("Validating credentials...")
	client, err := sdk.New(account, email, apiKey)
	if err != nil {
		return nil, &output.UserError{Message: err.Error()}
	}

	if _, err := client.ListProjects(cliCtx.Background()); err != nil {
		if sdk.IsUnauthorized(err) {
			return nil, &output.AuthError{
				Message: "Invalid credentials",
				Hint:    "Check your email and API key at Profile > API Key in DeployHQ",
			}
		}
		return nil, &output.InternalError{Message: "validate credentials", Cause: err}
	}

	creds := &auth.Credentials{Account: account, Email: email, APIKey: apiKey}
	if err := auth.Store(creds); err != nil {
		return nil, &output.InternalError{Message: "store credentials", Cause: err}
	}

	// Save account to global config so subsequent commands find it
	_ = config.Set(config.GlobalConfigPath(), "account", account)

	env.Status("")
	output.ColorGreen.Fprintf(env.Stderr, "Logged in as %s on %s.deployhq.com\n", email, account) //nolint:errcheck
	return creds, nil
}

func helloSignup(env *output.Envelope, reader *bufio.Reader) (*auth.Credentials, error) {
	env.Status("")

	fmt.Fprint(env.Stderr, "Email: ") //nolint:errcheck
	email, _ := reader.ReadString('\n')
	email = strings.TrimSpace(email)
	if email == "" {
		return nil, &output.UserError{Message: "Email is required"}
	}

	fmt.Fprint(env.Stderr, "Password: ") //nolint:errcheck
	pw, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Fprintln(env.Stderr) //nolint:errcheck
	if err != nil {
		return nil, &output.InternalError{Message: "read password", Cause: err}
	}
	password := strings.TrimSpace(string(pw))
	if password == "" {
		return nil, &output.UserError{Message: "Password is required"}
	}

	fmt.Fprint(env.Stderr, "Account name (optional, Enter to auto-generate): ") //nolint:errcheck
	accountName, _ := reader.ReadString('\n')
	accountName = strings.TrimSpace(accountName)

	env.Status("Creating account...")

	ua := cliUserAgent()
	result, err := sdk.Signup(sdk.SignupRequest{
		Email:       email,
		Password:    password,
		AccountName: accountName,
		Client:      ua,
	}, ua)
	if err != nil {
		return nil, err
	}

	creds := &auth.Credentials{
		Account: result.Account.Subdomain,
		Email:   email,
		APIKey:  result.APIKey,
	}
	if err := auth.Store(creds); err != nil {
		env.Warn("Could not save credentials: %v", err)
	}

	// Save account to global config so subsequent commands find it
	_ = config.Set(config.GlobalConfigPath(), "account", result.Account.Subdomain)

	env.Status("")
	output.ColorGreen.Fprintf(env.Stderr, "Account created: %s.deployhq.com\n", result.Account.Subdomain) //nolint:errcheck
	return creds, nil
}
