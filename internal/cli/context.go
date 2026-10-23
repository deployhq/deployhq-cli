// Package cli provides the command execution pipeline and shared context.
package cli

import (
	"context"
	"fmt"
	"strings"

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
	IsAgent  bool // true when invoked by an AI agent
	Version  string

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

// loadStoredCredentials reads a stored login. A variable so tests can stub the
// OS keychain.
var loadStoredCredentials = auth.LoadByAccount

// Credential sources reported by ResolveCredentials, in addition to the config
// layers ("flag", "env", "file").
const CredentialSourceStored = "stored"

// ResolvedCredentials are the credentials API commands use, and where each
// value came from ("flag", "env", "file" or "stored").
type ResolvedCredentials struct {
	Account string
	Email   string
	APIKey  string
	Sources map[string]string // "account", "email", "api_key" -> source
}

// NormalizeAccount accepts an account given as a bare subdomain, a hostname or
// a URL ("mycompany", "mycompany.deployhq.com", "https://mycompany.deployhq.com/")
// and returns just the subdomain. host is the configured DeployHQ host; empty
// means deployhq.com.
func NormalizeAccount(input, host string) string {
	s := strings.TrimSpace(input)
	s = strings.TrimPrefix(s, "https://")
	s = strings.TrimPrefix(s, "http://")
	if i := strings.IndexAny(s, "/?#"); i >= 0 {
		s = s[:i]
	}
	if host == "" {
		host = "deployhq.com"
	}
	return strings.TrimSuffix(s, "."+host)
}

// ResolveCredentials merges credentials from config (flags, env vars, config
// files) with the stored login: config values win, the stored login for the
// same account fills any gaps. It returns the same AuthError/UserError shapes
// that Client() would.
func (c *Context) ResolveCredentials() (*ResolvedCredentials, error) {
	r := &ResolvedCredentials{
		Account: NormalizeAccount(c.Config.Account, c.Config.Host),
		Email:   c.Config.Email,
		APIKey:  c.Config.APIKey,
		Sources: map[string]string{},
	}
	for key, val := range map[string]string{"account": r.Account, "email": r.Email, "api_key": r.APIKey} {
		if val != "" {
			r.Sources[key] = c.Config.Sources[key]
		}
	}

	if r.Account == "" || r.Email == "" || r.APIKey == "" {
		creds, loadErr := loadStoredCredentials(r.Account)
		if loadErr != nil {
			return nil, &output.AuthError{
				Message: "Not logged in",
				Hint:    auth.SignInHint(r.Account, c.Config.Host),
			}
		}
		if r.Account == "" {
			r.Account = creds.Account
			r.Sources["account"] = CredentialSourceStored
		}
		if r.Email == "" {
			r.Email = creds.Email
			r.Sources["email"] = CredentialSourceStored
		}
		if r.APIKey == "" {
			r.APIKey = creds.APIKey
			r.Sources["api_key"] = CredentialSourceStored
		}
	}

	if r.Account == "" {
		return nil, &output.UserError{
			Message: "Account not configured",
			Hint:    "Set via --account flag, DEPLOYHQ_ACCOUNT env var, or 'dhq config set account <name>'",
		}
	}
	return r, nil
}

// Credentials returns the resolved (account, email, apiKey) triplet; see
// ResolveCredentials.
//
// This is useful for code paths that need credentials but don't need
// the full SDK client — e.g. shelling out to a child process that
// reads them from environment variables.
func (c *Context) Credentials() (account, email, apiKey string, err error) {
	r, err := c.ResolveCredentials()
	if err != nil {
		return "", "", "", err
	}
	return r.Account, r.Email, r.APIKey, nil
}

// NewClient returns an SDK client for the given credentials, with the same
// options (user agent, base URL) as Client(). It is not cached.
func (c *Context) NewClient(account, email, apiKey string) (*sdk.Client, error) {
	return sdk.New(account, email, apiKey, c.clientOpts(account)...)
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

	client, err := sdk.New(account, email, apiKey, c.clientOpts(account)...)
	if err != nil {
		return nil, fmt.Errorf("create api client: %w", err)
	}

	c.client = client
	return c.client, nil
}

// PublicClient returns an SDK client for the public, no-auth endpoints
// (e.g. `dhq ip-ranges`, `dhq plans`). These endpoints return 200 without
// credentials, so email/apiKey are not required — but an account is still
// needed to know which subdomain to target.
//
// The account is resolved from config (flags/env/files) and, if absent
// there, from the stored auth credentials. It is NOT cached on the Context
// (that cache is reserved for the authenticated client).
func (c *Context) PublicClient() (*sdk.Client, error) {
	account, err := c.publicAccount()
	if err != nil {
		return nil, err
	}

	client, err := sdk.NewPublic(account, c.clientOpts(account)...)
	if err != nil {
		return nil, fmt.Errorf("create api client: %w", err)
	}
	return client, nil
}

// publicAccount resolves just the account subdomain for public endpoints,
// without requiring email/apiKey. A public endpoint still needs to know
// which account's subdomain to hit.
func (c *Context) publicAccount() (string, error) {
	if c.Config.Account != "" {
		return c.Config.Account, nil
	}
	// Fall back to any stored auth account, if available.
	if creds, err := auth.LoadByAccount(""); err == nil && creds.Account != "" {
		return creds.Account, nil
	}
	return "", &output.UserError{
		Message: "Account not configured",
		Hint:    "This is a public endpoint, but it still needs an account subdomain to query.\nSet via --account flag, DEPLOYHQ_ACCOUNT env var, or 'dhq config set account <name>'",
	}
}

// clientOpts builds the shared SDK options (user agent, base URL) used by
// both the authenticated and public clients.
func (c *Context) clientOpts(account string) []sdk.Option {
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
	return opts
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
