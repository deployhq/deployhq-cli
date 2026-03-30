// Package cli provides the command execution pipeline and shared context.
package cli

import (
	"context"
	"fmt"

	"github.com/deployhq/deployhq-cli/internal/auth"
	"github.com/deployhq/deployhq-cli/internal/config"
	"github.com/deployhq/deployhq-cli/internal/output"
	"github.com/deployhq/deployhq-cli/pkg/sdk"
)

// Context holds everything a command needs to execute.
// It is built once in the root command and passed down.
type Context struct {
	Config   *config.Config
	Envelope *output.Envelope
	Logger   *output.Logger
	IsAgent  bool // true when invoked by an AI agent

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

// Client returns the SDK client, creating it on first use.
// It merges credentials from config and auth store.
func (c *Context) Client() (*sdk.Client, error) {
	if c.client != nil {
		return c.client, nil
	}

	account := c.Config.Account
	email := c.Config.Email
	apiKey := c.Config.APIKey

	// Fill gaps from auth store
	if account == "" || email == "" || apiKey == "" {
		creds, err := auth.Load()
		if err != nil {
			return nil, &output.AuthError{
				Message: "Not logged in",
				Hint:    "Run 'deployhq auth login' to authenticate",
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
		return nil, &output.UserError{
			Message: "Account not configured",
			Hint:    "Set via --account flag, DEPLOYHQ_ACCOUNT env var, or 'deployhq config set account <name>'",
		}
	}

	opts := []sdk.Option{}
	ua := "deployhq-cli"
	if c.IsAgent {
		ua = "deployhq-cli (agent)"
	}
	opts = append(opts, sdk.WithUserAgent(ua))

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
		Hint:    "Set via --project flag, DEPLOYHQ_PROJECT env var, or .deployhq.toml",
	}
}

// Background returns a context.Context (for SDK calls).
func (c *Context) Background() context.Context {
	return context.Background()
}
