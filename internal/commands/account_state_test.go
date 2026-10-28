package commands

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/deployhq/deployhq-cli/internal/cli"
	"github.com/deployhq/deployhq-cli/internal/output"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// suspendedHandler answers like the API does for a suspended account: every
// request gets a 402 naming the suspension type.
func suspendedHandler(body string) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusPaymentRequired)
		_, _ = w.Write([]byte(body))
	}
}

const (
	trialEndedBody = `{"status":"suspended","suspension_type":"payment_issue","suspension_reason":"Automatic suspension for non-payment of subscription"}`
	otherSuspended = `{"status":"suspended","suspension_type":"infringement","suspension_reason":"Terms of service violation"}`
)

func TestCheckAuthStatus_SuspendedAccount(t *testing.T) {
	creds := &cli.ResolvedCredentials{
		Account: "acme", Email: "me@example.com", APIKey: "k",
		Sources: map[string]string{"account": "env", "email": "env", "api_key": "env"},
	}

	tests := []struct {
		name        string
		body        string
		wantMessage string
		wantNext    string
	}{
		{
			name:        "trial ended or payment outstanding",
			body:        trialEndedBody,
			wantMessage: "trial has ended",
			wantNext:    "Choose a plan",
		},
		{
			name:        "suspended by DeployHQ",
			body:        otherSuspended,
			wantMessage: "Terms of service violation",
			wantNext:    "support",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(suspendedHandler(tt.body))
			defer srv.Close()

			res := checkAuthStatus(t.Context(), newTestClient(t, srv), creds, "")
			assert.Equal(t, output.CodeAccountSuspended, res.Status)
			assert.Contains(t, res.Message, tt.wantMessage)
			assert.Contains(t, res.NextStep, tt.wantNext)
			assert.NotContains(t, res.NextStep, "Re-run 'dhq auth status'")
		})
	}
}

func TestLaunchDryRunCaps_SuspendedAccount_NamesBlocker(t *testing.T) {
	srv := httptest.NewServer(suspendedHandler(trialEndedBody))
	defer srv.Close()

	caps, err := launchDryRunCaps(t.Context(), newTestClient(t, srv))
	require.Error(t, err, "the preview must not report that the launch would go ahead")
	assert.Nil(t, caps)

	// Reported exactly as the real launch reports it.
	var acctErr *output.AccountError
	require.True(t, errors.As(output.ExplainAccountError(err), &acctErr))
	assert.Equal(t, output.CodeAccountSuspended, acctErr.Code)
}

func TestLaunchDryRunCaps_PlanWithoutAPIAccess_NamesBlocker(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"error":"api_access_restricted","message":"Upgrade your plan to gain access to the API"}`))
	}))
	defer srv.Close()

	caps, err := launchDryRunCaps(t.Context(), newTestClient(t, srv))
	require.Error(t, err)
	assert.Nil(t, caps)
	var acctErr *output.AccountError
	require.True(t, errors.As(output.ExplainAccountError(err), &acctErr))
	assert.Equal(t, output.CodeAPIAccessRestricted, acctErr.Code)
}

// A suspended account or a plan without API access stops launch at its first
// authenticated call. --json must carry a launch `reason` and `next_step`, like
// every other launch failure, with the exit code the account error has.
func TestLaunch_AccountRefusal_JSONReason(t *testing.T) {
	apiRestricted := func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"error":"api_access_restricted","message":"Upgrade your plan to gain access to the API"}`))
	}

	tests := []struct {
		name       string
		handler    http.HandlerFunc
		wantReason string
		wantExit   int
		wantURL    func(srvURL string) string
		wantNext   string
		wantAction string
	}{
		{
			name:       "trial ended or payment outstanding",
			handler:    suspendedHandler(trialEndedBody),
			wantReason: output.CodeAccountSuspended,
			wantExit:   output.ExitUserError,
			wantURL:    func(u string) string { return u + "/account/packages" },
			wantNext:   "Choose a plan",
			wantAction: "choose_plan",
		},
		{
			name:       "suspended by DeployHQ",
			handler:    suspendedHandler(otherSuspended),
			wantReason: output.CodeAccountSuspended,
			wantExit:   output.ExitUserError,
			wantURL:    func(string) string { return "https://www.deployhq.com/support" },
			wantNext:   "Contact DeployHQ support",
			wantAction: "contact_support",
		},
		{
			name:       "plan without API access",
			handler:    apiRestricted,
			wantReason: output.CodeAPIAccessRestricted,
			wantExit:   output.ExitAuthError,
			wantURL:    func(u string) string { return u + "/account/packages" },
			wantNext:   "Upgrade the plan",
			wantAction: "upgrade_plan",
		},
	}

	paths := map[string]func(*testing.T, *httptest.Server) error{
		"launch": func(t *testing.T, srv *httptest.Server) error {
			env, _, _ := testLaunchEnvelope()
			_, _, err := launchGetCaps(t.Context(), env, newTestClient(t, srv))
			return err
		},
		"dry-run": func(t *testing.T, srv *httptest.Server) error {
			_, err := launchDryRunCaps(t.Context(), newTestClient(t, srv))
			return err
		},
	}

	for _, tt := range tests {
		for pathName, run := range paths {
			t.Run(tt.name+"/"+pathName, func(t *testing.T) {
				srv := httptest.NewServer(tt.handler)
				defer srv.Close()

				err := run(t, srv)
				var le *launchError
				require.True(t, errors.As(err, &le), "want a launchError, got %T: %v", err, err)
				assert.Equal(t, tt.wantReason, le.Reason)

				env, stdout, _ := testLaunchEnvelopeJSON()
				result := writeLaunchError(env, launchConfig{}, le.Reason, err)
				assert.Equal(t, tt.wantExit, output.ClassifyError(result), "process exit code")

				var resp map[string]any
				require.NoError(t, json.Unmarshal(stdout.Bytes(), &resp))
				data := resp["data"].(map[string]any)
				url := tt.wantURL(srv.URL)
				assert.Equal(t, tt.wantReason, data["reason"])
				assert.Equal(t, float64(tt.wantExit), data["exit_code"])
				assert.Equal(t, false, data["retryable"])
				assert.Contains(t, data["next_step"], tt.wantNext)
				assert.Contains(t, data["next_step"], url)
				assert.Equal(t, url, data["details"].(map[string]any)["url"])
				assert.Contains(t, fmt.Sprint(data["recovery"]), tt.wantAction)
				assert.Contains(t, fmt.Sprint(data["recovery"]), url)
			})
		}
	}
}
