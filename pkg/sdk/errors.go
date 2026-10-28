package sdk

import (
	"fmt"
	"net/http"
	"strings"
)

// APIError represents an error returned by the DeployHQ API.
type APIError struct {
	StatusCode int      `json:"status"`
	Message    string   `json:"error,omitempty"`
	Errors     []string `json:"errors,omitempty"`
	// RetryAfter is the parsed Retry-After header (in seconds) accompanying a
	// 429 response, or 0 when absent/unparseable. Callers hitting a provisioning
	// rate limit should back off for this long before retrying.
	RetryAfter int `json:"retry_after,omitempty"`

	// ErrorCode, Detail and UpgradeURL carry the structured fields some error
	// responses include alongside the short `error` code: a machine-readable
	// `error_code`, the human-readable `message`, and an `upgrade_url` when the
	// account's plan is the cause. Empty when the response did not include them.
	ErrorCode  string `json:"error_code,omitempty"`
	Detail     string `json:"message,omitempty"`
	UpgradeURL string `json:"upgrade_url,omitempty"`

	// SuspensionType and SuspensionReason are set when the API refused the
	// request because the account is suspended (HTTP 402). SuspensionType is
	// e.g. "payment_issue" or "lapsed" (trial ended, or payment not received).
	SuspensionType   string `json:"suspension_type,omitempty"`
	SuspensionReason string `json:"suspension_reason,omitempty"`

	// AccountURL is the scheme and host of the account the failed request was
	// sent to (e.g. "https://acme.deployhq.com"). Lets callers link to the
	// account's own billing pages without re-deriving the account subdomain.
	AccountURL string `json:"-"`
}

// suspendedMessage is the Message recorded for a 402 account-suspended
// response, which carries no `error` field of its own.
const suspendedMessage = "account suspended"

// paymentSuspensionTypes are the suspension types the account owner can
// resolve themselves from the billing pages (trial ended, or a payment failed).
var paymentSuspensionTypes = map[string]bool{"payment_issue": true, "lapsed": true}

func (e *APIError) Error() string {
	// Prefer the human-readable message when the `error` field is only a code
	// (e.g. "api_access_restricted") and the response explained it.
	if e.Detail != "" && (e.Message == "" || e.Message == e.ErrorCode) {
		return fmt.Sprintf("deployhq api: %d %s", e.StatusCode, e.Detail)
	}
	if e.SuspensionType != "" && e.Message == suspendedMessage {
		return fmt.Sprintf("deployhq api: %d %s (%s)", e.StatusCode, e.Message, e.SuspensionType)
	}
	if e.Message != "" {
		return fmt.Sprintf("deployhq api: %d %s", e.StatusCode, e.Message)
	}
	if len(e.Errors) == 1 {
		return fmt.Sprintf("deployhq api: %d %s", e.StatusCode, e.Errors[0])
	}
	if len(e.Errors) > 1 {
		msg := fmt.Sprintf("deployhq api: %d validation failed", e.StatusCode)
		for _, err := range e.Errors {
			msg += "\n  - " + err
		}
		return msg
	}
	return fmt.Sprintf("deployhq api: %d %s", e.StatusCode, http.StatusText(e.StatusCode))
}

// IsNotFound returns true if the error is a 404.
func (e *APIError) IsNotFound() bool {
	return e.StatusCode == http.StatusNotFound
}

// IsUnauthorized returns true if the error is a 401.
func (e *APIError) IsUnauthorized() bool {
	return e.StatusCode == http.StatusUnauthorized
}

// IsForbidden returns true if the error is a 403.
func (e *APIError) IsForbidden() bool {
	return e.StatusCode == http.StatusForbidden
}

// IsEmailVerificationRequired reports the 403 the API returns for every request
// made with an API key while the account's email is not yet verified. It is
// deliberately distinct from a generic 403 (AccessDenied) — which must still
// surface as a real error.
func (e *APIError) IsEmailVerificationRequired() bool {
	return e.StatusCode == http.StatusForbidden && e.Message == "email_verification_required"
}

// IsAPIAccessRestricted reports whether the request was refused because the
// account's plan does not include API access. It deliberately does not match
// other refusals that share the same error code (such as an action reserved
// for the account administrator): only those that name the plan.
func (e *APIError) IsAPIAccessRestricted() bool {
	if e.StatusCode != http.StatusForbidden || e.Message != "api_access_restricted" {
		return false
	}
	return strings.Contains(strings.ToLower(e.Detail), "plan")
}

// IsAccountSuspended reports whether the request was refused because the
// account is suspended (HTTP 402). An ended trial is reported this way too.
func (e *APIError) IsAccountSuspended() bool {
	return e.StatusCode == http.StatusPaymentRequired
}

// IsPaymentSuspension reports whether the account is suspended for a reason
// the owner can resolve from the billing pages: the trial ended, or a payment
// failed. Other suspensions need DeployHQ support.
func (e *APIError) IsPaymentSuspension() bool {
	return e.IsAccountSuspended() && paymentSuspensionTypes[e.SuspensionType]
}

// IsPaymentMethodRequired reports whether a hosting-provisioning request was
// refused because the account has no accepted payment method on file. The
// response carries no dedicated code, so this matches the message text.
func (e *APIError) IsPaymentMethodRequired() bool {
	if e.StatusCode != http.StatusUnprocessableEntity {
		return false
	}
	return strings.Contains(strings.ToLower(e.Error()), "payment method is required")
}

// IsValidationError returns true if the error is a 422.
func (e *APIError) IsValidationError() bool {
	return e.StatusCode == http.StatusUnprocessableEntity
}

// IsRateLimited returns true if the error is a 429. For metered-resource
// provisioning this is the per-account provisioning rate limit — deliberately
// distinct from the 422 cap/kill-switch — and is safe to retry after backing
// off for RetryAfter seconds.
func (e *APIError) IsRateLimited() bool {
	return e.StatusCode == http.StatusTooManyRequests
}

// IsServerError returns true if the error is a 5xx.
func (e *APIError) IsServerError() bool {
	return e.StatusCode >= 500
}

// IsNotFound checks whether err is an APIError with status 404.
func IsNotFound(err error) bool {
	if apiErr, ok := err.(*APIError); ok {
		return apiErr.IsNotFound()
	}
	return false
}

// IsUnauthorized checks whether err is an APIError with status 401.
func IsUnauthorized(err error) bool {
	if apiErr, ok := err.(*APIError); ok {
		return apiErr.IsUnauthorized()
	}
	return false
}

// IsForbidden checks whether err is an APIError with status 403.
func IsForbidden(err error) bool {
	if apiErr, ok := err.(*APIError); ok {
		return apiErr.IsForbidden()
	}
	return false
}

// IsRateLimited checks whether err is an APIError with status 429.
func IsRateLimited(err error) bool {
	if apiErr, ok := err.(*APIError); ok {
		return apiErr.IsRateLimited()
	}
	return false
}
