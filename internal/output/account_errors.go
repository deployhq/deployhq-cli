package output

import (
	"errors"
	"fmt"

	"github.com/deployhq/deployhq-cli/pkg/sdk"
)

// Error codes for account-level refusals. These are not problems with the
// command or the credentials: the account itself cannot do what was asked
// until someone changes its plan or billing.
const (
	CodeAPIAccessRestricted   = "api_access_restricted"
	CodeAccountSuspended      = "account_suspended"
	CodePaymentMethodRequired = "payment_method_required"
)

// supportURL is where to go when a suspension cannot be resolved self-serve.
const supportURL = "https://www.deployhq.com/support"

// AccountError explains an API refusal caused by the account's plan or
// billing state. It names the cause and links to the page that resolves it.
// It wraps the original *sdk.APIError, so exit-code classification is unchanged.
type AccountError struct {
	Code    string
	Message string
	Hint    string
	// URL is the page that resolves the problem (plans, payment details or
	// support). Empty only if no link can be built.
	URL    string
	Action string // recovery action name for URL, e.g. "add_payment_method"
	Cause  *sdk.APIError
}

func (e *AccountError) Error() string {
	if e.Hint != "" {
		return fmt.Sprintf("%s\n\nHint: %s", e.Message, e.Hint)
	}
	return e.Message
}

func (e *AccountError) Unwrap() error { return e.Cause }

// Recovery returns the structured recovery actions for --json output.
func (e *AccountError) Recovery() []Breadcrumb {
	if e.URL == "" {
		return nil
	}
	return []Breadcrumb{{Action: e.Action, URL: e.URL}}
}

// ExplainAccountError returns an *AccountError when err is an API refusal
// caused by the account's plan or billing state (no API access on the plan,
// account suspended or trial ended, no payment method for a Managed VPS), and err
// unchanged otherwise.
func ExplainAccountError(err error) error {
	var already *AccountError
	if errors.As(err, &already) {
		return err
	}
	var apiErr *sdk.APIError
	if !errors.As(err, &apiErr) {
		return err
	}

	page := func(path string) string {
		if apiErr.AccountURL == "" {
			return ""
		}
		return apiErr.AccountURL + path
	}
	at := func(what, url string) string {
		if url == "" {
			return what + " in your DeployHQ account"
		}
		return what + " at " + url
	}

	switch {
	case apiErr.IsAPIAccessRestricted():
		url := apiErr.UpgradeURL
		if url == "" {
			url = page("/account/packages")
		}
		return &AccountError{
			Code:    CodeAPIAccessRestricted,
			Message: "This DeployHQ account's plan does not include API access, so dhq cannot use it.",
			Hint:    at("Upgrade the plan", url) + ", then re-run the command. Logging in again will not help.",
			URL:     url,
			Action:  "upgrade_plan",
			Cause:   apiErr,
		}

	case apiErr.IsPaymentSuspension():
		url := page("/account/packages")
		return &AccountError{
			Code:    CodeAccountSuspended,
			Message: "This DeployHQ account is suspended because its trial has ended or a payment is outstanding, so every API request is refused.",
			Hint:    at("Choose a plan or settle the outstanding payment", url) + ", then re-run the command.",
			URL:     url,
			Action:  "choose_plan",
			Cause:   apiErr,
		}

	case apiErr.IsAccountSuspended():
		msg := "This DeployHQ account is suspended, so every API request is refused."
		if apiErr.SuspensionReason != "" {
			msg = fmt.Sprintf("This DeployHQ account is suspended (%s), so every API request is refused.", apiErr.SuspensionReason)
		}
		return &AccountError{
			Code:    CodeAccountSuspended,
			Message: msg,
			Hint:    "This cannot be resolved from the CLI or the billing pages. Contact DeployHQ support: " + supportURL,
			URL:     supportURL,
			Action:  "contact_support",
			Cause:   apiErr,
		}

	case apiErr.IsPaymentMethodRequired():
		url := page("/account/payment_details")
		return &AccountError{
			Code:    CodePaymentMethodRequired,
			Message: "A payment method is required before this account can provision a Managed VPS.",
			Hint:    at("Add a card under Payment Details", url) + ", then re-run the command.",
			URL:     url,
			Action:  "add_payment_method",
			Cause:   apiErr,
		}
	}
	return err
}
