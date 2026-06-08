package commands

import (
	"errors"
	"fmt"
	"net"
	"strings"
	"syscall"
	"testing"

	"github.com/deployhq/deployhq-cli/internal/output"
	"github.com/deployhq/deployhq-cli/pkg/sdk"
)

func TestClassifyHelloValidateErr(t *testing.T) {
	tests := []struct {
		name    string
		in      error
		wantNil bool
		wantTyp string // class of returned typed error: "auth", "user", "network"
	}{
		{"401 invalid credentials", &sdk.APIError{StatusCode: 401, Message: "Unauthorized"}, false, "auth"},
		{"403 access denied", &sdk.APIError{StatusCode: 403, Message: "AccessDenied"}, false, "auth"},
		{"403 api_access_restricted", &sdk.APIError{StatusCode: 403, Message: "api_access_restricted"}, false, "auth"},
		{"404 not found", &sdk.APIError{StatusCode: 404, Message: "Not found"}, false, "user"},
		{"500 server error returns nil so caller passes through", &sdk.APIError{StatusCode: 500, Message: "boom"}, true, ""},
		{"422 validation returns nil so caller passes through", &sdk.APIError{StatusCode: 422, Message: "bad"}, true, ""},
		{"network OpError classifies as network", &net.OpError{Op: "dial", Net: "tcp", Err: syscall.ECONNREFUSED}, false, "network"},
		{"wrapped 403 via fmt.Errorf", fmt.Errorf("outer: %w", &sdk.APIError{StatusCode: 403}), false, "auth"},
		{"generic error returns nil", errors.New("???"), true, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := classifyHelloValidateErr(tt.in)
			if tt.wantNil {
				if got != nil {
					t.Fatalf("expected nil, got %T %v", got, got)
				}
				return
			}
			switch tt.wantTyp {
			case "auth":
				if _, ok := got.(*output.AuthError); !ok {
					t.Errorf("expected *AuthError, got %T %v", got, got)
				}
			case "user":
				if _, ok := got.(*output.UserError); !ok {
					t.Errorf("expected *UserError, got %T %v", got, got)
				}
			case "network":
				if _, ok := got.(*output.NetworkError); !ok {
					t.Errorf("expected *NetworkError, got %T %v", got, got)
				}
			}
		})
	}
}

func TestValidateHelloLoginInputs(t *testing.T) {
	tests := []struct {
		name           string
		account        string
		email          string
		apiKey         string
		wantErr        bool
		wantHeadline   string
		wantHintSubstr string
	}{
		{
			name:    "all set",
			account: "acct", email: "u@e.com", apiKey: "k",
			wantErr: false,
		},
		{
			name:    "empty account",
			account: "", email: "u@e.com", apiKey: "k",
			wantErr: true, wantHeadline: "Account is required",
			wantHintSubstr: "subdomain",
		},
		{
			name:    "empty email",
			account: "acct", email: "", apiKey: "k",
			wantErr: true, wantHeadline: "Email is required",
			wantHintSubstr: "sign in",
		},
		{
			// Regression guard: the original bug was sdk.New returning a raw
			// "deployhq: api key is required" message with no guidance.
			name:    "empty api key",
			account: "acct", email: "u@e.com", apiKey: "",
			wantErr: true, wantHeadline: "API key is required",
			wantHintSubstr: "Profile → API Key",
		},
		{
			// First failure wins so users fix one thing at a time.
			name:    "account checked before email",
			account: "", email: "", apiKey: "",
			wantErr: true, wantHeadline: "Account is required",
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			err := validateHelloLoginInputs(tt.account, tt.email, tt.apiKey)
			if !tt.wantErr {
				if err != nil {
					t.Fatalf("expected nil, got %v", err)
				}
				return
			}
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			userErr, ok := err.(*output.UserError)
			if !ok {
				t.Fatalf("expected *UserError, got %T %v", err, err)
			}
			// Headline must survive telemetry sanitization (first line of err.Error()).
			firstLine := strings.SplitN(userErr.Error(), "\n", 2)[0]
			if firstLine != tt.wantHeadline {
				t.Errorf("headline = %q, want %q", firstLine, tt.wantHeadline)
			}
			if tt.wantHintSubstr != "" && !strings.Contains(userErr.Hint, tt.wantHintSubstr) {
				t.Errorf("hint %q does not contain %q", userErr.Hint, tt.wantHintSubstr)
			}
		})
	}
}
