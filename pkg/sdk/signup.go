package sdk

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// SignupRequest is the payload for creating a new DeployHQ account.
// TermsAccepted must be true — the API rejects requests where it is false or absent.
// Client should be set to "dhq-cli" so the backend can attribute signups to the CLI funnel.
type SignupRequest struct {
	Email           string `json:"email"`
	Password        string `json:"password"`
	AccountName     string `json:"account_name,omitempty"`
	FullName        string `json:"full_name,omitempty"`
	Package         string `json:"package,omitempty"`
	Coupon          string `json:"coupon,omitempty"`
	NewsletterOptIn bool   `json:"newsletter_opt_in,omitempty"`
	SignupSource    string `json:"signup_source,omitempty"`
	// Client identifies the signup origin. Use "dhq-cli" for CLI-originated signups.
	Client string `json:"client,omitempty"`
	// TermsAccepted must be true. The API (signups_controller.rb:32) rejects the
	// request when this field is absent or false.
	TermsAccepted bool `json:"terms_accepted"`
}

// SignupResponse is the response from creating a new account.
// EmailVerified may be false for new signups — the account and api_key are still
// valid and usable; verification is advisory only. A 422 error mentioning
// "two-factor" signals an existing account with 2FA — use TwoFactorError to
// distinguish this case and redirect the user to browser-based login.
type SignupResponse struct {
	Account struct {
		Subdomain string `json:"subdomain"`
		Name      string `json:"name"`
	} `json:"account"`
	APIKey string `json:"api_key"`
	// EmailVerified is false when the signup email has not yet been confirmed.
	// It is non-blocking: the api_key is returned regardless (signup_service.rb:262,346).
	EmailVerified bool `json:"email_verified"`
	SSHPublicKey  struct {
		PublicKey   string `json:"public_key"`
		Fingerprint string `json:"fingerprint"`
	} `json:"ssh_public_key"`
}

// Signup creates a new DeployHQ account. This does not require authentication.
//
// userAgent is optional; when empty it defaults to "deployhq-cli".
// signupURL is optional; when empty it defaults to "https://api.deployhq.com/api/v1/signup".
//
// The caller must set req.TermsAccepted = true and req.Client = "dhq-cli".
//
// Returns TwoFactorError when the API responds with 422 and the error message
// indicates an existing account with 2FA enabled — the caller should redirect
// the user to browser-based signup/login instead of retrying headlessly.
func Signup(req SignupRequest, userAgent, signupURL string) (*SignupResponse, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal signup request: %w", err)
	}

	if userAgent == "" {
		userAgent = "deployhq-cli"
	}
	if signupURL == "" {
		signupURL = "https://api.deployhq.com/api/v1/signup"
	}

	httpReq, err := http.NewRequest(http.MethodPost, signupURL, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create signup request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("User-Agent", userAgent)

	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("signup request failed: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck

	respBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusCreated {
		// 422 with a two-factor error message means the email belongs to an existing
		// account that has 2FA enabled — the user must log in via browser.
		if resp.StatusCode == http.StatusUnprocessableEntity {
			var errResp struct {
				Errors []string `json:"errors"`
				Error  string   `json:"error"`
			}
			if json.Unmarshal(respBody, &errResp) == nil {
				combined := errResp.Error
				for _, e := range errResp.Errors {
					combined += " " + e
				}
				if strings.Contains(strings.ToLower(combined), "two-factor") ||
					strings.Contains(strings.ToLower(combined), "two_factor") ||
					strings.Contains(strings.ToLower(combined), "2fa") {
					return nil, &TwoFactorError{Message: strings.TrimSpace(combined)}
				}
			}
		}

		apiErr := &APIError{StatusCode: resp.StatusCode}
		var errResp struct {
			Errors []string `json:"errors"`
		}
		if json.Unmarshal(respBody, &errResp) == nil && len(errResp.Errors) > 0 {
			apiErr.Errors = errResp.Errors
		} else {
			apiErr.Message = string(respBody)
		}
		return nil, apiErr
	}

	var result SignupResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("decode signup response: %w", err)
	}
	return &result, nil
}
