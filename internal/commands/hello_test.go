package commands

import (
	"errors"
	"fmt"
	"net"
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
