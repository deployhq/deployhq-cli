package sdk

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// apiErrorFor runs one request against a server that always answers with the
// given status and body, and returns the resulting *APIError.
func apiErrorFor(t *testing.T, status int, body string) *APIError {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(server.Close)

	c := newTestClient(t, server)
	_, err := c.ListProjects(context.Background(), nil)
	require.Error(t, err)
	var apiErr *APIError
	require.True(t, errors.As(err, &apiErr))
	return apiErr
}

// The response for an API key whose account plan has no API access.
const planNoAPIBody = `{"error":"api_access_restricted","message":"Upgrade your plan to gain access to the API",` +
	`"upgrade_url":"https://acme.deployhq.com/account/packages","error_code":"api_access_restricted",` +
	`"hint":"Upgrade your plan to access this feature"}`

func TestParseAPIError_PlanWithoutAPIAccess(t *testing.T) {
	apiErr := apiErrorFor(t, http.StatusForbidden, planNoAPIBody)

	// Message keeps its existing meaning (the short `error` value) so code
	// comparing it keeps working; the rest lands in the new fields.
	assert.Equal(t, "api_access_restricted", apiErr.Message)
	assert.Equal(t, "api_access_restricted", apiErr.ErrorCode)
	assert.Equal(t, "Upgrade your plan to gain access to the API", apiErr.Detail)
	assert.Equal(t, "https://acme.deployhq.com/account/packages", apiErr.UpgradeURL)
	assert.True(t, apiErr.IsAPIAccessRestricted())
	assert.False(t, apiErr.IsAccountSuspended())
	assert.Equal(t, "deployhq api: 403 Upgrade your plan to gain access to the API", apiErr.Error())
}

func TestParseAPIError_AdminOnlyIsNotAPIAccessRestricted(t *testing.T) {
	// Same error code, different cause: not a plan problem.
	apiErr := apiErrorFor(t, http.StatusForbidden,
		`{"error":"api_access_restricted","message":"Only the account administrator can manage your account.",`+
			`"upgrade_url":"https://acme.deployhq.com/account/packages","error_code":"api_access_restricted"}`)
	assert.False(t, apiErr.IsAPIAccessRestricted())
}

func TestParseAPIError_AccountSuspended(t *testing.T) {
	apiErr := apiErrorFor(t, http.StatusPaymentRequired,
		`{"status":"suspended","suspension_type":"lapsed","suspension_reason":null}`)

	assert.True(t, apiErr.IsAccountSuspended())
	assert.True(t, apiErr.IsPaymentSuspension())
	assert.Equal(t, "lapsed", apiErr.SuspensionType)
	// Previously the raw JSON body became the message.
	assert.Equal(t, "deployhq api: 402 account suspended (lapsed)", apiErr.Error())
	assert.NotContains(t, apiErr.Error(), "{")
	assert.NotEmpty(t, apiErr.AccountURL)
}

func TestParseAPIError_NonPaymentSuspension(t *testing.T) {
	apiErr := apiErrorFor(t, http.StatusPaymentRequired,
		`{"status":"suspended","suspension_type":"other","suspension_reason":"Terms of service"}`)
	assert.True(t, apiErr.IsAccountSuspended())
	assert.False(t, apiErr.IsPaymentSuspension())
	assert.Equal(t, "Terms of service", apiErr.SuspensionReason)
}

func TestParseAPIError_PaymentMethodRequired(t *testing.T) {
	const msg = "A payment method is required before you can provision a hosted resource. " +
		"Add a card in your account's Billing settings, under Payment Details, then try again."

	// As a model validation error on server create.
	apiErr := apiErrorFor(t, http.StatusUnprocessableEntity, `{"base":["`+msg+`"]}`)
	assert.True(t, apiErr.IsPaymentMethodRequired())

	// As a plain error on hosted-resource re-provisioning.
	apiErr = apiErrorFor(t, http.StatusUnprocessableEntity, `{"error":"`+msg+`"}`)
	assert.True(t, apiErr.IsPaymentMethodRequired())

	// An unrelated validation error does not match.
	apiErr = apiErrorFor(t, http.StatusUnprocessableEntity, `{"name":["can't be blank"]}`)
	assert.False(t, apiErr.IsPaymentMethodRequired())
}

func TestParseAPIError_RecordsAccountURL(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusPaymentRequired)
		_, _ = w.Write([]byte(`{"status":"suspended","suspension_type":"payment_issue"}`))
	}))
	defer server.Close()

	c := newTestClient(t, server)
	_, err := c.ListProjects(context.Background(), nil)
	var apiErr *APIError
	require.True(t, errors.As(err, &apiErr))
	assert.Equal(t, server.URL, apiErr.AccountURL)
}

func TestParseAPIError_EmailVerificationUnchanged(t *testing.T) {
	apiErr := apiErrorFor(t, http.StatusForbidden, `{"error":"email_verification_required"}`)
	assert.True(t, apiErr.IsEmailVerificationRequired())
	assert.Equal(t, "deployhq api: 403 email_verification_required", apiErr.Error())
}

// `dhq api` goes through Client.Do. It must ask for JSON, otherwise a
// suspended account is answered with a redirect to the plans page instead of
// the 402 body the account-error mapping relies on.
func TestDo_SendsAcceptJSONAndSurfacesSuspension(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Accept") != "application/json" {
			http.Redirect(w, r, "/account/packages", http.StatusFound)
			return
		}
		w.WriteHeader(http.StatusPaymentRequired)
		_, _ = w.Write([]byte(`{"status":"suspended","suspension_type":"lapsed"}`))
	}))
	defer server.Close()

	c := newTestClient(t, server)
	var out any
	err := c.Do(context.Background(), http.MethodGet, "/profile", nil, &out)
	var apiErr *APIError
	require.True(t, errors.As(err, &apiErr), "got %v", err)
	assert.True(t, apiErr.IsPaymentSuspension())
}
