package output

import (
	"encoding/json"
	"errors"
	"fmt"
	"testing"

	"github.com/deployhq/deployhq-cli/pkg/sdk"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func planNoAPIErr() *sdk.APIError {
	return &sdk.APIError{
		StatusCode: 403,
		Message:    "api_access_restricted",
		ErrorCode:  "api_access_restricted",
		Detail:     "Upgrade your plan to gain access to the API",
		UpgradeURL: "https://acme.deployhq.com/account/packages",
		AccountURL: "https://acme.deployhq.com",
	}
}

func errorData(t *testing.T, err error) map[string]any {
	t.Helper()
	raw, mErr := json.Marshal(ErrorResponseFromErr(err))
	require.NoError(t, mErr)
	var out map[string]any
	require.NoError(t, json.Unmarshal(raw, &out))
	return out["data"].(map[string]any)
}

func TestExplainAccountError_PlanWithoutAPIAccess(t *testing.T) {
	err := ExplainAccountError(planNoAPIErr())
	var acct *AccountError
	require.True(t, errors.As(err, &acct))
	assert.Equal(t, CodeAPIAccessRestricted, acct.Code)
	assert.Contains(t, acct.Message, "plan does not include API access")
	assert.Contains(t, acct.Error(), "https://acme.deployhq.com/account/packages")
	// Still a 403 for exit-code purposes.
	assert.Equal(t, ExitAuthError, ClassifyError(err))

	data := errorData(t, planNoAPIErr())
	assert.Equal(t, CodeAPIAccessRestricted, data["code"])
	assert.Equal(t, float64(ExitAuthError), data["exit_code"])
	assert.Equal(t, false, data["retryable"])
	assert.Equal(t, "https://acme.deployhq.com/account/packages", data["doc_url"])
	recovery := data["recovery"].([]any)
	require.Len(t, recovery, 1)
	step := recovery[0].(map[string]any)
	assert.Equal(t, "upgrade_plan", step["action"])
	assert.Equal(t, "https://acme.deployhq.com/account/packages", step["url"])
	// Logging in again cannot fix a plan restriction.
	assert.NotContains(t, fmt.Sprint(recovery), "dhq auth login")
	_, hasCmd := step["cmd"]
	assert.False(t, hasCmd, "a URL recovery step has no cmd")
}

func TestExplainAccountError_TrialEndedOrPaymentSuspension(t *testing.T) {
	for _, typ := range []string{"lapsed", "payment_issue"} {
		apiErr := &sdk.APIError{StatusCode: 402, Message: "account suspended", SuspensionType: typ,
			AccountURL: "https://acme.deployhq.com"}
		data := errorData(t, apiErr)
		assert.Equal(t, CodeAccountSuspended, data["code"], typ)
		assert.Contains(t, data["error"], "trial has ended", typ)
		assert.Equal(t, "https://acme.deployhq.com/account/packages", data["doc_url"], typ)
		assert.Equal(t, float64(ExitUserError), data["exit_code"], typ)
		step := data["recovery"].([]any)[0].(map[string]any)
		assert.Equal(t, "choose_plan", step["action"])
	}
}

func TestExplainAccountError_OtherSuspensionPointsToSupport(t *testing.T) {
	apiErr := &sdk.APIError{StatusCode: 402, Message: "account suspended", SuspensionType: "other",
		SuspensionReason: "Terms of service", AccountURL: "https://acme.deployhq.com"}
	data := errorData(t, apiErr)
	assert.Equal(t, CodeAccountSuspended, data["code"])
	assert.Contains(t, data["error"], "Terms of service")
	assert.NotContains(t, data["doc_url"], "/account/packages")
	step := data["recovery"].([]any)[0].(map[string]any)
	assert.Equal(t, "contact_support", step["action"])
}

func TestExplainAccountError_PaymentMethodRequired(t *testing.T) {
	apiErr := &sdk.APIError{StatusCode: 422, AccountURL: "https://acme.deployhq.com",
		Errors: []string{"base A payment method is required before you can provision a hosted resource."}}
	err := ExplainAccountError(fmt.Errorf("create server: %w", apiErr))
	var acct *AccountError
	require.True(t, errors.As(err, &acct))
	assert.Equal(t, CodePaymentMethodRequired, acct.Code)
	assert.Equal(t, "A payment method is required before this account can provision this hosting.", acct.Message)
	// It does not know the target, so it must not name one.
	assert.NotContains(t, acct.Message, "Managed VPS")
	assert.Equal(t, "https://acme.deployhq.com/account/payment_details", acct.URL)
	assert.Contains(t, acct.Error(), "https://acme.deployhq.com/account/payment_details")
}

func TestExplainAccountError_NoAccountURLStillExplains(t *testing.T) {
	apiErr := &sdk.APIError{StatusCode: 402, Message: "account suspended", SuspensionType: "lapsed"}
	var acct *AccountError
	require.True(t, errors.As(ExplainAccountError(apiErr), &acct))
	assert.Empty(t, acct.URL)
	assert.Nil(t, acct.Recovery())
	assert.Contains(t, acct.Hint, "in your DeployHQ account")
}

func TestExplainAccountError_LeavesOtherErrorsAlone(t *testing.T) {
	for _, in := range []error{
		errors.New("boom"),
		&sdk.APIError{StatusCode: 403, Message: "AccessDenied"},
		&sdk.APIError{StatusCode: 403, Message: "api_access_restricted", ErrorCode: "api_access_restricted",
			Detail: "Only the account administrator can manage your account."},
		&sdk.APIError{StatusCode: 422, Errors: []string{"name can't be blank"}},
	} {
		assert.Same(t, in, ExplainAccountError(in))
	}
	// A plain 403 keeps its existing recovery.
	data := errorData(t, &sdk.APIError{StatusCode: 403, Message: "AccessDenied"})
	assert.Equal(t, "auth_error", data["code"])
}
