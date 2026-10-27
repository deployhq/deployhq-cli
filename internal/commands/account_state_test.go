package commands

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/deployhq/deployhq-cli/internal/cli"
	"github.com/deployhq/deployhq-cli/internal/output"
	"github.com/stretchr/testify/assert"
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
