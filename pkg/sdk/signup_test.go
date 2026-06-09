package sdk

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSignup_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)

		var body SignupRequest
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		assert.Equal(t, "user@example.com", body.Email)
		assert.Equal(t, "dhq-cli", body.Client, "client must be 'dhq-cli'")
		assert.True(t, body.TermsAccepted, "terms_accepted must be true")

		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(SignupResponse{
			Account: struct {
				Subdomain string `json:"subdomain"`
				Name      string `json:"name"`
			}{Subdomain: "mycompany", Name: "My Company"},
			APIKey:        "test-api-key",
			EmailVerified: true,
		})
	}))
	defer server.Close()

	req := SignupRequest{
		Email:         "user@example.com",
		Password:      "secret",
		TermsAccepted: true,
		Client:        "dhq-cli",
	}
	result, err := Signup(req, "deployhq-cli", server.URL+"/api/v1/signup")
	require.NoError(t, err)
	assert.Equal(t, "mycompany", result.Account.Subdomain)
	assert.Equal(t, "test-api-key", result.APIKey)
	assert.True(t, result.EmailVerified)
}

func TestSignup_EmailNotVerified(t *testing.T) {
	// email_verified: false is non-blocking — api_key is still returned.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(SignupResponse{
			Account: struct {
				Subdomain string `json:"subdomain"`
				Name      string `json:"name"`
			}{Subdomain: "myco"},
			APIKey:        "key-123",
			EmailVerified: false,
		})
	}))
	defer server.Close()

	req := SignupRequest{Email: "user@example.com", Password: "secret", TermsAccepted: true, Client: "dhq-cli"}
	result, err := Signup(req, "", server.URL+"/api/v1/signup")
	require.NoError(t, err)
	assert.Equal(t, "key-123", result.APIKey, "api_key must be present even when email is unverified")
	assert.False(t, result.EmailVerified)
}

func TestSignup_TwoFactorError(t *testing.T) {
	// 422 with two-factor message → TwoFactorError, not generic APIError.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnprocessableEntity)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"errors": []string{"Email is already taken. Log in with two-factor authentication via the browser."},
		})
	}))
	defer server.Close()

	req := SignupRequest{Email: "existing@example.com", Password: "secret", TermsAccepted: true, Client: "dhq-cli"}
	_, err := Signup(req, "", server.URL+"/api/v1/signup")
	require.Error(t, err)

	tfErr, ok := err.(*TwoFactorError)
	require.True(t, ok, "expected TwoFactorError, got %T: %v", err, err)
	assert.Contains(t, tfErr.Error(), "two-factor")
}

func TestSignup_TwoFactorError_2FA(t *testing.T) {
	// Variant with "2fa" in error body (case-insensitive).
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnprocessableEntity)
		_ = json.NewEncoder(w).Encode(map[string]string{
			"error": "Account requires 2FA verification",
		})
	}))
	defer server.Close()

	req := SignupRequest{Email: "existing@example.com", Password: "secret", TermsAccepted: true, Client: "dhq-cli"}
	_, err := Signup(req, "", server.URL+"/api/v1/signup")
	require.Error(t, err)
	_, ok := err.(*TwoFactorError)
	require.True(t, ok, "expected TwoFactorError for 2FA error, got %T", err)
}

func TestSignup_ValidationError(t *testing.T) {
	// Non-2FA 422 → regular APIError.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnprocessableEntity)
		_ = json.NewEncoder(w).Encode(map[string][]string{
			"errors": {"Password is too short"},
		})
	}))
	defer server.Close()

	req := SignupRequest{Email: "user@example.com", Password: "x", TermsAccepted: true, Client: "dhq-cli"}
	_, err := Signup(req, "", server.URL+"/api/v1/signup")
	require.Error(t, err)

	// Must NOT be a TwoFactorError
	_, isTFA := err.(*TwoFactorError)
	assert.False(t, isTFA, "non-2FA 422 must not be a TwoFactorError")

	apiErr, ok := err.(*APIError)
	require.True(t, ok)
	assert.Equal(t, 422, apiErr.StatusCode)
}

func TestSignup_TermsAccepted_SentInPayload(t *testing.T) {
	// Verify the terms_accepted field is always serialised as true when set.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]interface{}
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		assert.Equal(t, true, body["terms_accepted"], "terms_accepted must be true in payload")

		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(SignupResponse{
			Account: struct {
				Subdomain string `json:"subdomain"`
				Name      string `json:"name"`
			}{Subdomain: "co"},
			APIKey: "k",
		})
	}))
	defer server.Close()

	req := SignupRequest{Email: "u@x.com", Password: "p", TermsAccepted: true, Client: "dhq-cli"}
	_, err := Signup(req, "", server.URL+"/api/v1/signup")
	require.NoError(t, err)
}

func TestSignup_UserAgentDefault(t *testing.T) {
	// When userAgent is empty, the default "deployhq-cli" should be used.
	var capturedUA string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedUA = r.Header.Get("User-Agent")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(SignupResponse{
			Account: struct {
				Subdomain string `json:"subdomain"`
				Name      string `json:"name"`
			}{Subdomain: "co"},
			APIKey: "k",
		})
	}))
	defer server.Close()

	req := SignupRequest{Email: "u@x.com", Password: "p", TermsAccepted: true}
	_, err := Signup(req, "", server.URL+"/api/v1/signup")
	require.NoError(t, err)
	assert.Equal(t, "deployhq-cli", capturedUA, "empty userAgent must use default 'deployhq-cli'")
}
