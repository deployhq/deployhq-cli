package cli

import (
	"errors"
	"testing"

	"github.com/deployhq/deployhq-cli/internal/auth"
	"github.com/deployhq/deployhq-cli/internal/config"
	"github.com/deployhq/deployhq-cli/internal/output"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// stubStore replaces the OS keychain with a fixed set of stored logins. The
// default profile ("" key) mimics LoadByAccount's fallback: it is returned for
// an empty account, or when it belongs to the requested account.
func stubStore(t *testing.T, defaultProfile *auth.Credentials, named map[string]*auth.Credentials) *[]string {
	t.Helper()
	var calls []string
	orig := loadStoredCredentials
	t.Cleanup(func() { loadStoredCredentials = orig })
	loadStoredCredentials = func(account string) (*auth.Credentials, error) {
		calls = append(calls, account)
		if c, ok := named[account]; ok {
			return c, nil
		}
		if defaultProfile != nil && (account == "" || defaultProfile.Account == account) {
			return defaultProfile, nil
		}
		return nil, errors.New("not logged in")
	}
	return &calls
}

func envConfig(account, email, key, host string) *config.Config {
	return &config.Config{
		Account: account, Email: email, APIKey: key, Host: host,
		Sources: map[string]string{"account": "env", "email": "env", "api_key": "env"},
	}
}

var otherStoredLogin = &auth.Credentials{Account: "sg", Email: "someone@example.com", APIKey: "stored-key"}

func TestResolveCredentials_EnvWinsOverDifferentStoredDefault(t *testing.T) {
	calls := stubStore(t, otherStoredLogin, nil)

	// The account is given as a full hostname, which the CLI otherwise tolerates.
	c := &Context{Config: envConfig("qa.staging.example.com", "qa@example.com", "env-key", "staging.example.com")}
	r, err := c.ResolveCredentials()
	require.NoError(t, err)

	assert.Equal(t, "qa", r.Account, "account is normalised to the subdomain")
	assert.Equal(t, "qa@example.com", r.Email)
	assert.Equal(t, "env-key", r.APIKey)
	assert.Equal(t, map[string]string{"account": "env", "email": "env", "api_key": "env"}, r.Sources)
	assert.Empty(t, *calls, "complete env credentials must not consult the stored logins")
}

func TestResolveCredentials_GapsFilledOnlyFromSameAccount(t *testing.T) {
	stubStore(t, otherStoredLogin, nil)

	// Account from env, but no stored login for it: never fall back to another
	// account's stored login.
	c := &Context{Config: envConfig("qa.deployhq.com", "", "", "")}
	_, err := c.ResolveCredentials()
	var authErr *output.AuthError
	require.True(t, errors.As(err, &authErr), "got %v", err)
}

func TestResolveCredentials_StoredLoginFillsGaps(t *testing.T) {
	stubStore(t, otherStoredLogin, map[string]*auth.Credentials{
		"qa": {Account: "qa", Email: "qa@example.com", APIKey: "qa-key"},
	})

	c := &Context{Config: envConfig("https://qa.deployhq.com/", "", "", "")}
	r, err := c.ResolveCredentials()
	require.NoError(t, err)
	assert.Equal(t, "qa", r.Account)
	assert.Equal(t, "qa-key", r.APIKey)
	assert.Equal(t, "env", r.Sources["account"])
	assert.Equal(t, CredentialSourceStored, r.Sources["api_key"])
}

func TestResolveCredentials_NothingConfiguredUsesDefaultStoredLogin(t *testing.T) {
	stubStore(t, otherStoredLogin, nil)

	c := &Context{Config: &config.Config{}}
	r, err := c.ResolveCredentials()
	require.NoError(t, err)
	assert.Equal(t, "sg", r.Account)
	for _, k := range []string{"account", "email", "api_key"} {
		assert.Equal(t, CredentialSourceStored, r.Sources[k])
	}
}

func TestNormalizeAccount(t *testing.T) {
	assert.Equal(t, "qa", NormalizeAccount("qa.staging.example.com", "staging.example.com"))
	assert.Equal(t, "qa", NormalizeAccount("https://qa.deployhq.com/x", ""))
	assert.Equal(t, "qa", NormalizeAccount(" qa ", ""))
}
