// Package cli provides the command execution pipeline and shared context.
package cli

import (
	"context"
	"fmt"

	"github.com/deployhq/deployhq-cli/internal/auth"
	"github.com/deployhq/deployhq-cli/internal/config"
	"github.com/deployhq/deployhq-cli/internal/harness"
	"github.com/deployhq/deployhq-cli/internal/output"
	"github.com/deployhq/deployhq-cli/pkg/sdk"
)

// Context holds everything a command needs to execute.
// It is built once in the root command and passed down.
type Context struct {
	Config   *config.Config
	Envelope *output.Envelope
	Logger   *output.Logger
	IsAgent bool // true when invoked by an AI agent
	Version string

	// Lazy-initialized API client (only when a command needs it)
	client *sdk.Client
}

// NewContext builds a CLI context from config.
func NewContext(cfg *config.Config, env *output.Envelope, logger *output.Logger) *Context {
	return &Context{
		Config:   cfg,
		Envelope: env,
		Logger:   logger,
	}
}

// Credentials returns the resolved (account, email, apiKey) triplet,
// merging values from config (flags/env/files) with the auth store. It
// returns the same AuthError/UserError shapes that Client() would.
//
// This is useful for code paths that need credentials but don't need
// the full SDK client — e.g. shelling out to a child process that
// reads them from environment variables.
func (c *Context) Credentials() (account, email, apiKey string, err error) {
	account = c.Config.Account
	email = c.Config.Email
	apiKey = c.Config.APIKey

	if account == "" || email == "" || apiKey == "" {
		creds, loadErr := auth.LoadByAccount(account)
		if loadErr != nil {
			return "", "", "", &output.AuthError{
				Message: "Not logged in",
				Hint: "Authenticate first:\n" +
					"  dhq auth login                                                    (interactive)\n" +
					"  export DEPLOYHQ_ACCOUNT=… DEPLOYHQ_EMAIL=… DEPLOYHQ_API_KEY=…    (CI / agents)",
			}
		}
		if account == "" {
			account = creds.Account
		}
		if email == "" {
			email = creds.Email
		}
		if apiKey == "" {
			apiKey = creds.APIKey
		}
	}

	if account == "" {
		return "", "", "", &output.UserError{
			Message: "Account not configured",
			Hint:    "Set via --account flag, DEPLOYHQ_ACCOUNT env var, or 'dhq config set account <name>'",
		}
	}
	return account, email, apiKey, nil
}

// Client returns the SDK client, creating it on first use.
// It merges credentials from config and auth store.
func (c *Context) Client() (*sdk.Client, error) {
	if c.client != nil {
		return c.client, nil
	}

	account, email, apiKey, err := c.Credentials()
	if err != nil {
		return nil, err
	}

	opts := []sdk.Option{}
	agent := harness.AgentInfo{}
	if c.IsAgent {
		agent = harness.Detect()
	}
	v := c.Version
	if v == "" {
		v = "dev"
	}
	opts = append(opts, sdk.WithUserAgent(harness.UserAgent(v, agent)))

	if baseURL := c.Config.BaseURL(account); baseURL != "" {
		opts = append(opts, sdk.WithBaseURL(baseURL))
	}

	client, err := sdk.New(account, email, apiKey, opts...)
	if err != nil {
		return nil, fmt.Errorf("create api client: %w", err)
	}

	c.client = client
	return c.client, nil
}

// RequireProject returns the project identifier, or a UserError if not set.
func (c *Context) RequireProject() (string, error) {
	if c.Config.Project != "" {
		return c.Config.Project, nil
	}
	return "", &output.UserError{
		Message: "No project specified",
		Hint: "Find your project identifier with 'dhq projects list', then set it:\n" +
			"  dhq config set project <identifier>      (persist for this directory)\n" +
			"  --project <identifier>                   (pass per command)\n" +
			"  export DEPLOYHQ_PROJECT=<identifier>     (set for the shell session)",
	}
}

// Background returns a context.Context (for SDK calls).
func (c *Context) Background() context.Context {
	return context.Background()
}
