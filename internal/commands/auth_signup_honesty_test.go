package commands

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/deployhq/deployhq-cli/internal/auth"
	"github.com/deployhq/deployhq-cli/internal/cli"
	"github.com/deployhq/deployhq-cli/internal/config"
	"github.com/deployhq/deployhq-cli/internal/output"
	"github.com/deployhq/deployhq-cli/pkg/sdk"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/zalando/go-keyring"
)

// emailUnverifiedHandler answers every request the way the API does for an
// account whose email address has not been verified yet.
func emailUnverifiedHandler(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusForbidden)
	_, _ = w.Write([]byte(`{"error":"email_verification_required"}`))
}

func withTestCLIContext(t *testing.T, nonInteractive bool) *bytes.Buffer {
	t.Helper()
	var stderr bytes.Buffer
	orig := cliCtx
	t.Cleanup(func() { cliCtx = orig })
	cliCtx = &cli.Context{
		Config: &config.Config{},
		Envelope: &output.Envelope{
			Stdout: &bytes.Buffer{}, Stderr: &stderr,
			Logger: &output.Logger{}, NonInteractive: nonInteractive,
		},
	}
	return &stderr
}

// ── Sign-in guidance ─────────────────────────────────────────────────────────

func TestAuthLogin_NonInteractive_PointsToOwnTerminalNotKeyFlag(t *testing.T) {
	for name, opts := range map[string]*AuthLoginOptions{
		"nothing given":          {},
		"account and email only": {Account: "acme", Email: "me@example.com"},
	} {
		t.Run(name, func(t *testing.T) {
			withTestCLIContext(t, true)
			err := runAuthLogin(opts)
			require.Error(t, err)

			var authErr *output.AuthError
			require.True(t, errors.As(err, &authErr), "got %T", err)
			assert.Contains(t, authErr.Hint, "separate terminal window")
			assert.Contains(t, authErr.Hint, "Settings → Security → API Credentials")
			assert.Contains(t, authErr.Hint, "DEPLOYHQ_API_KEY")
			assert.NotContains(t, authErr.Hint, "--api-key", "must not suggest passing the key through the agent")
			assert.NotContains(t, authErr.Hint, "Profile")
		})
	}
}

func TestSignInHint_UsesAccountURL(t *testing.T) {
	assert.Contains(t, auth.SignInHint("acme", ""), "https://acme.deployhq.com/security")
	assert.Contains(t, auth.SignInHint("", ""), "https://<your-account>.deployhq.com/security")
	assert.Contains(t, auth.APIKeyLocation("acme", "deployhq.dev"), "https://acme.deployhq.dev/security")
}

func TestSignInHint_MentionsSignupForPeopleWithoutAnAccount(t *testing.T) {
	hint := auth.SignInHint("", "")
	assert.Contains(t, hint, "dhq signup")
	assert.NotContains(t, hint, "dhq hello")
	assert.Contains(t, hint, "Never paste an API key into an AI chat")
}

func TestNotLoggedInNextStep_MentionsSignup(t *testing.T) {
	assert.Contains(t, notLoggedInNextStep, "dhq auth login")
	assert.Contains(t, notLoggedInNextStep, "dhq signup")
	// Sign-up just succeeded when the credentials could not be saved: signing
	// up again is not the fix.
	assert.NotContains(t, credentialsNotSavedNextStep, "dhq signup")
	assert.Contains(t, credentialsNotSavedNextStep, "dhq auth login")
}

// ── auth status ──────────────────────────────────────────────────────────────

func TestCheckAuthStatus(t *testing.T) {
	creds := &cli.ResolvedCredentials{
		Account: "acme", Email: "me@example.com", APIKey: "k",
		Sources: map[string]string{"account": "env", "email": "env", "api_key": "env"},
	}

	tests := []struct {
		name       string
		handler    http.HandlerFunc
		wantStatus string
		wantNext   string
	}{
		{
			name: "working credentials",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				_, _ = w.Write([]byte(`{"email":"me@example.com"}`))
			},
			wantStatus: authStatusAuthenticated,
		},
		{
			name:       "email not verified",
			handler:    emailUnverifiedHandler,
			wantStatus: authStatusEmailUnverified,
			wantNext:   "verification link",
		},
		{
			name: "revoked key",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusUnauthorized)
				_, _ = w.Write([]byte(`{"error":"Unauthorized"}`))
			},
			wantStatus: authStatusInvalidCredential,
			wantNext:   "DEPLOYHQ_API_KEY",
		},
		{
			// What the API returns for a wrong or revoked API key.
			name: "wrong key: 403 AccessDenied",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "text/plain")
				w.WriteHeader(http.StatusForbidden)
				_, _ = w.Write([]byte(`{"error":"AccessDenied"}`))
			},
			wantStatus: authStatusInvalidCredential,
			wantNext:   "DEPLOYHQ_API_KEY", // credentials came from the environment
		},
		{
			// What the API returns for an account subdomain that doesn't exist.
			name: "unknown account: 404 Not found",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "text/plain")
				w.WriteHeader(http.StatusNotFound)
				_, _ = w.Write([]byte(`Not found`))
			},
			wantStatus: authStatusInvalidCredential,
		},
		{
			name: "plan without API access",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusForbidden)
				_, _ = w.Write([]byte(`{"error":"api_access_restricted","message":"Upgrade your plan to gain access to the API"}`))
			},
			wantStatus: authStatusAPIRestricted,
			wantNext:   "/account/packages",
		},
		{
			name: "server error is unconfirmed",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusBadGateway)
			},
			wantStatus: authStatusUnconfirmed,
			wantNext:   "connection",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(tt.handler)
			defer srv.Close()

			res := checkAuthStatus(t.Context(), newTestClient(t, srv), creds, "")
			assert.Equal(t, tt.wantStatus, res.Status)
			assert.Equal(t, "acme", res.Account)
			assert.Equal(t, "env", res.Source)
			if tt.wantNext != "" {
				assert.Contains(t, res.NextStep, tt.wantNext)
			}
			if tt.wantStatus != authStatusAuthenticated {
				assert.NotEmpty(t, res.Message)
			}
		})
	}
}

// ── launch --dry-run ─────────────────────────────────────────────────────────

func TestLaunchDryRunCaps_EmailUnverified_NamesBlocker(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(emailUnverifiedHandler))
	defer srv.Close()

	caps, err := launchDryRunCaps(t.Context(), newTestClient(t, srv))
	require.Error(t, err)
	assert.Nil(t, caps)
	var le *launchError
	require.True(t, errors.As(err, &le))
	assert.Equal(t, reasonEmailVerificationRequired, le.Reason)

	// The --json preview must report the failure, not "ok": true.
	env, stdout, _ := testLaunchEnvelopeJSON()
	_ = writeLaunchError(env, launchConfig{dryRun: true}, reasonEmailVerificationRequired, err)
	var resp struct {
		OK   bool `json:"ok"`
		Data struct {
			Reason string `json:"reason"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(stdout.Bytes(), &resp))
	assert.False(t, resp.OK)
	assert.Equal(t, reasonEmailVerificationRequired, resp.Data.Reason)
}

func TestLaunchDryRunCaps_OtherErrorsDegrade(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	caps, err := launchDryRunCaps(t.Context(), newTestClient(t, srv))
	require.NoError(t, err)
	require.NotNil(t, caps)
}

// ── Sign-up ──────────────────────────────────────────────────────────────────

func TestConfirmSignupTerms(t *testing.T) {
	reader := func(s string) *bufio.Reader { return bufio.NewReader(strings.NewReader(s)) }

	t.Run("non-interactive without --accept-terms refuses", func(t *testing.T) {
		env, _ := newTestEnv(true)
		err := confirmSignupTerms(env, reader(""), false)
		var userErr *output.UserError
		require.True(t, errors.As(err, &userErr))
		assert.Contains(t, userErr.Hint, "--accept-terms")
		assert.Contains(t, userErr.Hint, termsURL)
	})
	t.Run("non-interactive with --accept-terms proceeds", func(t *testing.T) {
		env, _ := newTestEnv(true)
		assert.NoError(t, confirmSignupTerms(env, reader(""), true))
	})
	t.Run("interactive yes", func(t *testing.T) {
		env, buf := newTestEnv(false)
		assert.NoError(t, confirmSignupTerms(env, reader("y\n"), false))
		assert.Contains(t, buf.String(), termsURL, "the person is shown the terms before answering")
	})
	t.Run("interactive empty answer is not acceptance", func(t *testing.T) {
		env, _ := newTestEnv(false)
		assert.Error(t, confirmSignupTerms(env, reader("\n"), false))
	})
	t.Run("interactive no", func(t *testing.T) {
		env, _ := newTestEnv(false)
		assert.Error(t, confirmSignupTerms(env, reader("n\n"), false))
	})
}

func signupResult(subdomain string, verified bool) *sdk.SignupResponse {
	r := &sdk.SignupResponse{APIKey: "k", EmailVerified: verified}
	r.Account.Subdomain = subdomain
	return r
}

func TestReportSignupOutcome_RenamedAndUnverified(t *testing.T) {
	env, buf := newTestEnv(false)
	reportSignupOutcome(env, "acme", signupResult("acme6", false), "me@example.com", "deployhq.com")
	out := buf.String()
	assert.Contains(t, out, `"acme"`)
	assert.Contains(t, out, "acme6.deployhq.com")
	assert.Contains(t, out, "Check your inbox")
	assert.Contains(t, out, "me@example.com")
}

func TestReportSignupOutcome_NothingToSay(t *testing.T) {
	for name, requested := range map[string]string{"same name": "Acme", "auto-generated": ""} {
		t.Run(name, func(t *testing.T) {
			env, buf := newTestEnv(false)
			reportSignupOutcome(env, requested, signupResult("acme", true), "me@example.com", "deployhq.com")
			assert.Empty(t, buf.String())
		})
	}
}

func TestNewSignupOutput_JSON(t *testing.T) {
	out := newSignupOutput(signupResult("acme6", false), "acme", "me@example.com")
	b, err := json.Marshal(out)
	require.NoError(t, err)

	var got map[string]interface{}
	require.NoError(t, json.Unmarshal(b, &got))
	assert.Equal(t, true, got["account_name_changed"])
	assert.Equal(t, "acme", got["requested_account_name"])
	assert.Contains(t, got["next_step"], "verification link")
	assert.Equal(t, "acme6", got["account"].(map[string]interface{})["subdomain"], "API fields are still present")

	same, err := json.Marshal(newSignupOutput(signupResult("acme", true), "acme", "me@example.com"))
	require.NoError(t, err)
	assert.NotContains(t, string(same), "account_name_changed")
	assert.NotContains(t, string(same), "next_step")
}

// ── Root help ────────────────────────────────────────────────────────────────

func TestRootHelp_PointsAgentsToSetupInstructions(t *testing.T) {
	cmd := NewRootCmd("test")
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetArgs([]string{"--help"})
	require.NoError(t, cmd.Execute())
	assert.Contains(t, stdout.String(), "https://www.deployhq.com/agents.md")
	assert.Contains(t, stdout.String(), "install DeployHQ's skill with 'dhq skills install' and follow it")
}

func TestDescribeCredentialSource(t *testing.T) {
	env := map[string]string{"account": "env", "email": "env", "api_key": "env"}
	assert.Equal(t, "env", credentialSource(env))
	assert.Contains(t, describeCredentialSource(env), "environment variables")

	mixed := map[string]string{"account": "env", "email": "stored", "api_key": "stored"}
	assert.Equal(t, "mixed", credentialSource(mixed))
	assert.Contains(t, describeCredentialSource(mixed), "account from environment variables")
	assert.Contains(t, describeCredentialSource(mixed), "api_key from stored login")
}

func TestInvalidCredentialsNextStep_BySource(t *testing.T) {
	stored := &cli.ResolvedCredentials{Account: "acme", Sources: map[string]string{"api_key": cli.CredentialSourceStored}}
	assert.Contains(t, invalidCredentialsNextStep(stored, ""), "separate terminal window")
	assert.Contains(t, invalidCredentialsNextStep(stored, ""), "https://acme.deployhq.com/security")

	env := &cli.ResolvedCredentials{Account: "acme", Sources: map[string]string{"api_key": "env"}}
	assert.Contains(t, invalidCredentialsNextStep(env, ""), "DEPLOYHQ_API_KEY")
	assert.NotContains(t, invalidCredentialsNextStep(env, ""), "connection")

	file := &cli.ResolvedCredentials{Account: "acme", Sources: map[string]string{"api_key": "file"}}
	assert.Contains(t, invalidCredentialsNextStep(file, ""), "'dhq config show'")
	assert.NotContains(t, invalidCredentialsNextStep(file, ""), "configure", "'dhq configure' is the interactive wizard")
}

// isolateCredentialStore keeps a test away from the developer's real logins:
// an in-memory keyring and an empty home directory for the file fallback.
// MockInit starts a fresh store each call, so no test sees another's logins.
func isolateCredentialStore(t *testing.T) {
	t.Helper()
	keyring.MockInit()
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	// os.UserHomeDir reads USERPROFILE on Windows.
	t.Setenv("USERPROFILE", dir)
}

// withJSONCLIContext is withTestCLIContext for --json runs; it returns stdout.
func withJSONCLIContext(t *testing.T) *bytes.Buffer {
	t.Helper()
	var stdout bytes.Buffer
	orig := cliCtx
	t.Cleanup(func() { cliCtx = orig })
	cliCtx = &cli.Context{
		Config: &config.Config{},
		Envelope: &output.Envelope{
			Stdout: &stdout, Stderr: io.Discard,
			Logger: &output.Logger{}, NonInteractive: true, JSONMode: true,
		},
	}
	return &stdout
}

func TestAuthStatus_JSON_NotLoggedIn(t *testing.T) {
	isolateCredentialStore(t)
	stdout := withJSONCLIContext(t)

	cmd := newAuthStatusCmd()
	require.NoError(t, cmd.RunE(cmd, nil), "not being signed in is a status, not a failure")

	var res map[string]any
	dec := json.NewDecoder(stdout)
	require.NoError(t, dec.Decode(&res), "--json must write a JSON document, got %q", stdout.String())
	assert.False(t, dec.More(), "exactly one JSON document")
	assert.Equal(t, authStatusNotLoggedIn, res["status"])
	assert.NotEmpty(t, res["message"])
	assert.Contains(t, res["next_step"], "dhq auth login")
	assert.Contains(t, res["next_step"], "separate terminal window")
}

func TestCheckAuthStatus_NetworkFailureIsUnconfirmed(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	client := newTestClient(t, srv)
	srv.Close() // connection refused from here on

	res := checkAuthStatus(t.Context(), client, &cli.ResolvedCredentials{Account: "acme"}, "")
	assert.Equal(t, authStatusUnconfirmed, res.Status)
	assert.Contains(t, res.NextStep, "connection")
}

func TestSignup_JSON_OmitsAPIKey(t *testing.T) {
	isolateCredentialStore(t)
	stdout := withJSONCLIContext(t)

	orig := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = orig })
	http.DefaultTransport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusCreated,
			Body: io.NopCloser(strings.NewReader(`{"account":{"subdomain":"acme","name":"Acme"},` +
				`"api_key":"secret-signup-key","email_verified":false,` +
				`"ssh_public_key":{"public_key":"ssh-ed25519 AAAA","fingerprint":"SHA256:x"}}`)),
			Header:  http.Header{"Content-Type": []string{"application/json"}},
			Request: r,
		}, nil
	})

	cmd := newSignupCmd()
	cmd.SetArgs([]string{"--email", "me@example.com", "--password", "pw", "--account-name", "acme", "--accept-terms"})
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	require.NoError(t, cmd.Execute())

	assert.NotContains(t, stdout.String(), "secret-signup-key")
	var resp struct {
		Data map[string]any `json:"data"`
	}
	require.NoError(t, json.Unmarshal(stdout.Bytes(), &resp), "got %q", stdout.String())
	assert.NotContains(t, resp.Data, "api_key")
	assert.Contains(t, resp.Data, "account")
	assert.Contains(t, resp.Data, "email_verified")
	assert.Contains(t, resp.Data, "ssh_public_key")

	stored, err := auth.LoadByAccount("acme")
	require.NoError(t, err)
	assert.Equal(t, "secret-signup-key", stored.APIKey, "the key is saved, not printed")
}

// stubSignupAPI answers the sign-up request with a new, unverified account
// named subdomain.
func stubSignupAPI(t *testing.T, subdomain string) {
	t.Helper()
	orig := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = orig })
	http.DefaultTransport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusCreated,
			Body: io.NopCloser(strings.NewReader(`{"account":{"subdomain":"` + subdomain + `","name":"New"},` +
				`"api_key":"new-key","email_verified":false,` +
				`"ssh_public_key":{"public_key":"ssh-ed25519 AAAA","fingerprint":"SHA256:x"}}`)),
			Header:  http.Header{"Content-Type": []string{"application/json"}},
			Request: r,
		}, nil
	})
}

func TestSignup_SwitchesGlobalConfigToNewAccount(t *testing.T) {
	isolateCredentialStore(t)
	withJSONCLIContext(t)
	require.NoError(t, config.Set(config.GlobalConfigPath(), "account", "older-account"))
	stubSignupAPI(t, "newco")

	cmd := newSignupCmd()
	cmd.SetArgs([]string{"--email", "me@example.com", "--password", "pw", "--accept-terms"})
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	require.NoError(t, cmd.Execute())

	assert.Equal(t, "newco", config.Get(config.GlobalConfigPath(), "account"),
		"commands after sign-up must use the new account, not the one configured before")
}

func TestLaunchEnsureAuth_NonInteractive_HintNormalisesHostnameAccount(t *testing.T) {
	isolateCredentialStore(t)
	withTestCLIContext(t, true)
	cliCtx.Config.Account = "acme.deployhq.com"

	_, _, err := launchEnsureAuth(t.Context(), cliCtx.Envelope, launchConfig{})
	require.Error(t, err)

	var authErr *output.AuthError
	require.True(t, errors.As(err, &authErr), "got %T", err)
	assert.Contains(t, authErr.Hint, "https://acme.deployhq.com/security")
	assert.NotContains(t, authErr.Hint, "deployhq.com.deployhq.com")
}

// withPipedCLIContext is a run with stdout piped and no output flag: JSONMode
// is false, but the envelope switches to JSON automatically, as it does for
// agents. It returns stdout.
func withPipedCLIContext(t *testing.T) *bytes.Buffer {
	t.Helper()
	stdout := withJSONCLIContext(t)
	cliCtx.Envelope.JSONMode = false
	cliCtx.Envelope.IsTTY = false
	require.True(t, cliCtx.Envelope.WantsJSON())
	return stdout
}

func TestAuthStatus_PipedOutputIsJSON_NotLoggedIn(t *testing.T) {
	isolateCredentialStore(t)
	stdout := withPipedCLIContext(t)

	cmd := newAuthStatusCmd()
	require.NoError(t, cmd.RunE(cmd, nil))

	var res map[string]any
	require.NoError(t, json.Unmarshal(stdout.Bytes(), &res), "piped output must be JSON, got %q", stdout.String())
	assert.Equal(t, authStatusNotLoggedIn, res["status"])
}

func TestAuthStatus_PipedOutputIsJSON_Authenticated(t *testing.T) {
	isolateCredentialStore(t)
	stdout := withPipedCLIContext(t)
	cliCtx.Config.Account = "acme"
	cliCtx.Config.Email = "me@example.com"
	cliCtx.Config.APIKey = "k"
	cliCtx.Config.Sources = map[string]string{"account": "env", "email": "env", "api_key": "env"}

	orig := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = orig })
	http.DefaultTransport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(`{"email":"me@example.com"}`)),
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Request:    r,
		}, nil
	})

	cmd := newAuthStatusCmd()
	require.NoError(t, cmd.RunE(cmd, nil))

	var res map[string]any
	require.NoError(t, json.Unmarshal(stdout.Bytes(), &res), "piped output must be JSON, got %q", stdout.String())
	assert.Equal(t, authStatusAuthenticated, res["status"])
	assert.Equal(t, "acme", res["account"])
}

// runJSONSignup runs 'dhq signup --json' against a stubbed sign-up API and
// returns the response's data.
func runJSONSignup(t *testing.T, stdout *bytes.Buffer) map[string]any {
	t.Helper()
	orig := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = orig })
	http.DefaultTransport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusCreated,
			Body: io.NopCloser(strings.NewReader(`{"account":{"subdomain":"acme","name":"Acme"},` +
				`"api_key":"secret-signup-key","email_verified":false}`)),
			Header:  http.Header{"Content-Type": []string{"application/json"}},
			Request: r,
		}, nil
	})

	cmd := newSignupCmd()
	cmd.SetArgs([]string{"--email", "me@example.com", "--password", "pw", "--account-name", "acme", "--accept-terms"})
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	require.NoError(t, cmd.Execute())

	assert.NotContains(t, stdout.String(), "secret-signup-key")
	var resp struct {
		Data map[string]any `json:"data"`
	}
	require.NoError(t, json.Unmarshal(stdout.Bytes(), &resp), "got %q", stdout.String())
	return resp.Data
}

func TestSignup_JSON_ReportsCredentialsSaved(t *testing.T) {
	isolateCredentialStore(t)
	data := runJSONSignup(t, withJSONCLIContext(t))
	assert.Equal(t, true, data["credentials_saved"])
	assert.NotContains(t, data["next_step"], "dhq auth login")
}

func TestSignup_JSON_ReportsCredentialSaveFailure(t *testing.T) {
	// No keyring, and a home directory that is a file, so neither store works.
	keyring.MockInitWithError(errors.New("keyring unavailable"))
	home := filepath.Join(t.TempDir(), "home")
	require.NoError(t, os.WriteFile(home, nil, 0o600))
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)

	data := runJSONSignup(t, withJSONCLIContext(t))
	assert.Equal(t, false, data["credentials_saved"])
	assert.Contains(t, data["next_step"], "could not be saved")
	assert.Contains(t, data["next_step"], "dhq auth login")
	assert.Contains(t, data["next_step"], "verification link", "the verify-email notice still applies")
}

func TestSignup_Text_CredentialSaveFailure_DoesNotClaimSaved(t *testing.T) {
	for _, verified := range []bool{true, false} {
		t.Run(fmt.Sprintf("email verified %v", verified), func(t *testing.T) {
			// No keyring, and a home directory that is a file, so neither store works.
			keyring.MockInitWithError(errors.New("keyring unavailable"))
			home := filepath.Join(t.TempDir(), "home")
			require.NoError(t, os.WriteFile(home, nil, 0o600))
			t.Setenv("HOME", home)
			t.Setenv("USERPROFILE", home)

			stderr := withTestCLIContext(t, true)
			cliCtx.Envelope.IsTTY = true
			orig := http.DefaultTransport
			t.Cleanup(func() { http.DefaultTransport = orig })
			http.DefaultTransport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode: http.StatusCreated,
					Body: io.NopCloser(strings.NewReader(fmt.Sprintf(`{"account":{"subdomain":"newco","name":"New"},`+
						`"api_key":"new-key","email_verified":%v,`+
						`"ssh_public_key":{"public_key":"ssh-ed25519 AAAA","fingerprint":"SHA256:x"}}`, verified))),
					Header:  http.Header{"Content-Type": []string{"application/json"}},
					Request: r,
				}, nil
			})

			cmd := newSignupCmd()
			cmd.SetArgs([]string{"--email", "me@example.com", "--password", "pw", "--account-name", "newco", "--accept-terms"})
			cmd.SetOut(io.Discard)
			cmd.SetErr(io.Discard)
			require.NoError(t, cmd.Execute())

			out := stderr.String()
			assert.NotContains(t, out, "Credentials saved")
			assert.Contains(t, out, credentialsNotSavedNextStep)
			if !verified {
				assert.Contains(t, out, "verified your email")
			}
		})
	}
}

func TestAuthStatus_JSON_NotLoggedIn_NamesConfiguredAccount(t *testing.T) {
	isolateCredentialStore(t)
	stdout := withJSONCLIContext(t)
	cliCtx.Config.Account = "acme"
	cliCtx.Config.Sources = map[string]string{"account": "env", "email": "default", "api_key": "default"}

	cmd := newAuthStatusCmd()
	require.NoError(t, cmd.RunE(cmd, nil))

	var res map[string]any
	require.NoError(t, json.Unmarshal(stdout.Bytes(), &res), "got %q", stdout.String())
	assert.Equal(t, authStatusNotLoggedIn, res["status"])
	assert.Equal(t, "acme", res["account"])
	assert.Equal(t, "env", res["source"])
	assert.Equal(t, map[string]any{"account": "env"}, res["sources"])
	assert.Contains(t, res["next_step"], "dhq auth login")
	assert.Contains(t, res["next_step"], "DEPLOYHQ_ACCOUNT")
}

// ── auth login: account prompt ───────────────────────────────────────────────

func TestPromptAccount_AsksAgainAfterEmptyAnswer(t *testing.T) {
	stderr := withTestCLIContext(t, false)
	account, err := promptAccount(cliCtx.Envelope, bufio.NewReader(strings.NewReader("\n  \nacme\n")))
	require.NoError(t, err)
	assert.Equal(t, "acme", account)
	assert.Equal(t, 3, strings.Count(stderr.String(), "Account subdomain"))
}

func TestPromptAccount_GivesUpAfterThreeEmptyAnswers(t *testing.T) {
	stderr := withTestCLIContext(t, false)
	_, err := promptAccount(cliCtx.Envelope, bufio.NewReader(strings.NewReader("\n\n\n\n")))
	var uerr *output.UserError
	require.True(t, errors.As(err, &uerr), "got %T: %v", err, err)
	assert.Equal(t, "Account is required", uerr.Message)
	assert.Equal(t, 3, strings.Count(stderr.String(), "Account subdomain"))
}

func TestPromptAccount_StopsAtEndOfInput(t *testing.T) {
	stderr := withTestCLIContext(t, false)
	_, err := promptAccount(cliCtx.Envelope, bufio.NewReader(strings.NewReader("")))
	var uerr *output.UserError
	require.True(t, errors.As(err, &uerr), "got %T: %v", err, err)
	assert.Equal(t, 1, strings.Count(stderr.String(), "Account subdomain"), "nothing more can be read")

	// A last line without a newline is still an answer.
	withTestCLIContext(t, false)
	account, err := promptAccount(cliCtx.Envelope, bufio.NewReader(strings.NewReader("acme")))
	require.NoError(t, err)
	assert.Equal(t, "acme", account)
}

// ── signup: next steps ───────────────────────────────────────────────────────

// A new account can't use the API until its email is verified, so sign-up
// must not suggest commands that fail until then as if they worked now.
func TestSignup_Unverified_NextStepsComeAfterVerification(t *testing.T) {
	args := []string{"--email", "me@example.com", "--password", "pw", "--account-name", "newco", "--accept-terms"}

	t.Run("text", func(t *testing.T) {
		isolateCredentialStore(t)
		stderr := withTestCLIContext(t, true)
		cliCtx.Envelope.IsTTY = true
		stubSignupAPI(t, "newco")

		cmd := newSignupCmd()
		cmd.SetArgs(args)
		cmd.SetOut(io.Discard)
		cmd.SetErr(io.Discard)
		require.NoError(t, cmd.Execute())

		out := stderr.String()
		assert.Contains(t, out, "Once you've verified your email")
		assert.Contains(t, out, "dhq projects create --name my-app")
		assert.NotContains(t, out, "Get started:")
	})

	t.Run("json", func(t *testing.T) {
		isolateCredentialStore(t)
		stdout := withJSONCLIContext(t)
		stubSignupAPI(t, "newco")

		cmd := newSignupCmd()
		cmd.SetArgs(args)
		cmd.SetOut(io.Discard)
		cmd.SetErr(io.Discard)
		require.NoError(t, cmd.Execute())

		var resp output.Response
		require.NoError(t, json.Unmarshal(stdout.Bytes(), &resp), "got %q", stdout.String())
		require.NotEmpty(t, resp.Breadcrumbs)
		assert.Equal(t, output.Breadcrumb{Action: "check_status", Cmd: "dhq auth status"}, resp.Breadcrumbs[0])
		for _, b := range resp.Breadcrumbs[1:] {
			assert.Contains(t, b.Action, "verified", "later steps wait for the email to be verified: %+v", b)
		}
	})
}

// An agent running 'dhq signup' must be told to hand it to the person, not
// which flag to pass a password with.
func TestSignup_NonInteractive_LeadsWithHandingOverToThePerson(t *testing.T) {
	for name, args := range map[string][]string{
		"nothing given": {},
		"email only":    {"--email", "me@example.com"},
	} {
		t.Run(name, func(t *testing.T) {
			blockNetwork(t)
			withTestCLIContext(t, true)

			cmd := newSignupCmd()
			cmd.SetArgs(args)
			cmd.SetOut(io.Discard)
			cmd.SetErr(io.Discard)
			err := cmd.Execute()

			var uerr *output.UserError
			require.True(t, errors.As(err, &uerr), "got %T: %v", err, err)
			assert.Equal(t, "Signing up needs the person at the keyboard: dhq can't ask for a password in non-interactive mode", uerr.Message)
			assert.Equal(t, signupInOwnTerminalHint, uerr.Hint)
			assert.NotContains(t, err.Error(), "--password", "must not suggest passing a password through the agent")
		})
	}
}
