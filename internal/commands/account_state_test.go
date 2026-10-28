package commands

import (
	"errors"
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
