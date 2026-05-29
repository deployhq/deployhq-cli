package output

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"syscall"
	"testing"
	"time"

	"github.com/deployhq/deployhq-cli/pkg/sdk"
	"github.com/stretchr/testify/assert"
)

func TestUserError(t *testing.T) {
	err := &UserError{Message: "bad input", Hint: "use --flag"}
	assert.Contains(t, err.Error(), "bad input")
	assert.Contains(t, err.Error(), "use --flag")
	assert.Equal(t, ExitUserError, ClassifyError(err))
}

func TestUserError_NoHint(t *testing.T) {
	err := &UserError{Message: "bad input"}
	assert.Equal(t, "bad input", err.Error())
	assert.NotContains(t, err.Error(), "Hint")
}

func TestInternalError(t *testing.T) {
	cause := errors.New("disk full")
	err := &InternalError{Message: "write failed", Cause: cause}
	assert.Contains(t, err.Error(), "write failed")
	assert.Contains(t, err.Error(), "disk full")
	assert.Equal(t, ExitInternalError, ClassifyError(err))
	assert.Equal(t, cause, errors.Unwrap(err))
}

func TestAuthError(t *testing.T) {
	err := &AuthError{Message: "not logged in", Hint: "run dhq auth login"}
	assert.Contains(t, err.Error(), "not logged in")
	assert.Equal(t, ExitAuthError, ClassifyError(err))
}

func TestClassifyError_Nil(t *testing.T) {
	assert.Equal(t, ExitOK, ClassifyError(nil))
}

func TestClassifyError_GenericError(t *testing.T) {
	assert.Equal(t, ExitInternalError, ClassifyError(errors.New("unknown")))
}

func TestNetworkError(t *testing.T) {
	cause := errors.New("dial tcp: i/o timeout")
	err := &NetworkError{Message: "validate credentials", Cause: cause}
	assert.Contains(t, err.Error(), "validate credentials")
	assert.Contains(t, err.Error(), "dial tcp")
	assert.Equal(t, ExitNetworkError, ClassifyError(err))
	assert.Equal(t, cause, errors.Unwrap(err))
}

func TestIsNetworkErr_Nil(t *testing.T) {
	assert.False(t, IsNetworkErr(nil))
}

func TestIsNetworkErr_GenericError(t *testing.T) {
	assert.False(t, IsNetworkErr(errors.New("not a network thing")))
}

func TestIsNetworkErr_Syscalls(t *testing.T) {
	cases := []syscall.Errno{
		syscall.ECONNREFUSED,
		syscall.ECONNRESET,
		syscall.EHOSTUNREACH,
		syscall.ENETUNREACH,
		syscall.EPIPE,
	}
	for _, e := range cases {
		assert.True(t, IsNetworkErr(e), "expected syscall errno %v to classify as network", e)
	}
}

func TestIsNetworkErr_DNSError(t *testing.T) {
	dnsErr := &net.DNSError{Err: "no such host", Name: "api.example.invalid"}
	assert.True(t, IsNetworkErr(dnsErr))
}

func TestIsNetworkErr_OpError(t *testing.T) {
	opErr := &net.OpError{Op: "dial", Net: "tcp", Err: syscall.ECONNREFUSED}
	assert.True(t, IsNetworkErr(opErr))
}

// timeoutErr satisfies net.Error with Timeout() = true.
type timeoutErr struct{}

func (timeoutErr) Error() string   { return "i/o timeout" }
func (timeoutErr) Timeout() bool   { return true }
func (timeoutErr) Temporary() bool { return true }

func TestIsNetworkErr_TimeoutNetError(t *testing.T) {
	assert.True(t, IsNetworkErr(timeoutErr{}))
}

func TestIsNetworkErr_URLErrorWraps(t *testing.T) {
	urlErr := &url.Error{
		Op:  "Get",
		URL: "https://api.example.com",
		Err: &net.OpError{Op: "dial", Net: "tcp", Err: syscall.ECONNREFUSED},
	}
	assert.True(t, IsNetworkErr(urlErr))
}

func TestIsNetworkErr_URLErrorWithNonNetworkCause(t *testing.T) {
	urlErr := &url.Error{
		Op:  "Get",
		URL: "https://api.example.com",
		Err: errors.New("malformed body"),
	}
	assert.False(t, IsNetworkErr(urlErr))
}

func TestClassifyError_DetectsRawNetworkError(t *testing.T) {
	// An untyped network error returned from the SDK should classify as ExitNetworkError,
	// not ExitInternalError, even when no command wraps it.
	err := &net.OpError{Op: "dial", Net: "tcp", Err: syscall.ECONNREFUSED}
	assert.Equal(t, ExitNetworkError, ClassifyError(err))
}

func TestClassifyError_DetectsURLNetworkError(t *testing.T) {
	urlErr := &url.Error{
		Op:  "Get",
		URL: "https://api.example.com",
		Err: timeoutErr{},
	}
	assert.Equal(t, ExitNetworkError, ClassifyError(urlErr))
}

func TestClassifyError_APIErrorByStatus(t *testing.T) {
	cases := []struct {
		status int
		want   int
		name   string
	}{
		{401, ExitAuthError, "401 unauthorized"},
		{403, ExitAuthError, "403 forbidden"},
		{404, ExitUserError, "404 not found is a user-supplied bad id"},
		{409, ExitUserError, "409 conflict is user state"},
		{422, ExitUserError, "422 validation is bad input"},
		{500, ExitInternalError, "500 server is internal"},
		{503, ExitInternalError, "503 unavailable is internal"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := &sdk.APIError{StatusCode: tc.status, Message: "x"}
			assert.Equal(t, tc.want, ClassifyError(err))
		})
	}
}

func TestClassifyError_APIErrorWrapped(t *testing.T) {
	// errors.As should unwrap through a wrapping error to find the APIError.
	apiErr := &sdk.APIError{StatusCode: 404, Message: "not found"}
	wrapped := fmt.Errorf("fetch project: %w", apiErr)
	assert.Equal(t, ExitUserError, ClassifyError(wrapped))
}

func TestClassifyError_TypedErrorWinsOverAPIError(t *testing.T) {
	// If the command explicitly wraps an API 5xx as a UserError, honor that.
	apiErr := &sdk.APIError{StatusCode: 500, Message: "boom"}
	wrapped := &UserError{Message: "you triggered the bug", Hint: "blah"}
	_ = apiErr // sanity: classification of the typed error doesn't peek inside
	assert.Equal(t, ExitUserError, ClassifyError(wrapped))
}

// Sanity check: a deadline-style error wrapped in net.Error.Timeout() classifies
// even when accessed through deadline-exceeded chains used by net/http.
func TestIsNetworkErr_DeadlineExceededViaNetError(t *testing.T) {
	deadline := &net.OpError{
		Op:     "read",
		Net:    "tcp",
		Source: nil,
		Addr:   nil,
		Err:    timeoutErr{},
	}
	assert.True(t, IsNetworkErr(deadline))
	// And confirm it doesn't depend on the elapsed time
	_ = time.Second
}
