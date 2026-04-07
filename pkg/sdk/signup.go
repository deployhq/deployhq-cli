package sdk

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// SignupRequest is the payload for creating a new DeployHQ account.
type SignupRequest struct {
	Email           string `json:"email"`
	Password        string `json:"password"`
	AccountName     string `json:"account_name,omitempty"`
	FullName        string `json:"full_name,omitempty"`
	Package         string `json:"package,omitempty"`
	Coupon          string `json:"coupon,omitempty"`
	NewsletterOptIn bool   `json:"newsletter_opt_in,omitempty"`
	SignupSource    string `json:"signup_source,omitempty"`
	Client          string `json:"client,omitempty"`
}

// SignupResponse is the response from creating a new account.
type SignupResponse struct {
	Account struct {
		Subdomain string `json:"subdomain"`
		Name      string `json:"name"`
	} `json:"account"`
	APIKey       string `json:"api_key"`
	SSHPublicKey struct {
		PublicKey   string `json:"public_key"`
		Fingerprint string `json:"fingerprint"`
	} `json:"ssh_public_key"`
}

// Signup creates a new DeployHQ account. This does not require authentication.
// userAgent is optional; when empty it defaults to "deployhq-cli".
// signupURL is optional; when empty it defaults to "https://api.deployhq.com/api/v1/signup".
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
