package commands

import (
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/deployhq/deployhq-cli/internal/output"
	"github.com/deployhq/deployhq-cli/pkg/sdk"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Every launch refusal exits with the documented code for its kind, not 2
// (internal error): 2 tells an agent the CLI itself broke.
func TestLaunchError_ExitCodePerReason(t *testing.T) {
	cases := []struct {
		reason string
		want   int
	}{
		{reasonPlanLimitReached, output.ExitUserError},
		{reasonAcceptCostRequired, output.ExitUserError},
		{reasonBetaEnrollRequired, output.ExitUserError},
		{reasonRepoUnreachable, output.ExitUserError},
		{reasonRateLimited, output.ExitUserError},
		{"payment_method_required", output.ExitUserError},
		{reasonSubdomainTaken, output.ExitConflictError},
		{reasonEmailVerificationRequired, output.ExitAuthError},
		{reasonAuthRequired, output.ExitAuthError},
		{reasonProvisionFailed, output.ExitInternalError},
		{reasonDeployFailed, output.ExitInternalError},
		{"some_future_reason", output.ExitInternalError},
	}
	for _, tc := range cases {
		t.Run(tc.reason, func(t *testing.T) {
			err := &launchError{Reason: tc.reason, Message: "refused"}
			assert.Equal(t, tc.want, output.ClassifyError(err))
		})
	}
}

// The constructors used by launch produce the documented codes too.
func TestLaunchError_ExitCodeFromConstructors(t *testing.T) {
	assert.Equal(t, output.ExitAuthError, output.ClassifyError(emailVerificationLaunchError()))

	rl := rateLimitLaunchError(&sdk.APIError{StatusCode: http.StatusTooManyRequests, RetryAfter: 5})
	require.NotNil(t, rl)
	assert.Equal(t, output.ExitUserError, output.ClassifyError(rl))
	assert.True(t, rl.Retryable, "rate_limited stays retryable")
}

// In --json the single error object's exit_code is the process exit code.
func TestWriteLaunchError_JSONExitCodeMatchesReason(t *testing.T) {
	cases := []struct {
		reason string
		want   int
	}{
		{reasonPlanLimitReached, output.ExitUserError},
		{reasonSubdomainTaken, output.ExitConflictError},
		{reasonEmailVerificationRequired, output.ExitAuthError},
		{reasonProvisionFailed, output.ExitInternalError},
	}
	for _, tc := range cases {
		t.Run(tc.reason, func(t *testing.T) {
			env, stdout, _ := testLaunchEnvelopeJSON()
			err := writeLaunchError(env, launchConfig{}, tc.reason, &launchError{Reason: tc.reason, Message: "refused"})
			assert.Equal(t, tc.want, output.ClassifyError(err))

			var resp map[string]interface{}
			require.NoError(t, json.NewDecoder(stdout).Decode(&resp))
			data := resp["data"].(map[string]interface{})
			assert.Equal(t, tc.reason, data["reason"])
			assert.Equal(t, float64(tc.want), data["exit_code"])
		})
	}
}

// A provision_failed reported for a raw API error keeps that error's code: a
// 422 from the API is something the person must change.
func TestWriteLaunchError_ProvisionFailedRawAPIErrorKeepsItsCode(t *testing.T) {
	env, stdout, _ := testLaunchEnvelopeJSON()
	err := writeLaunchError(env, launchConfig{}, reasonProvisionFailed, &sdk.APIError{StatusCode: http.StatusUnprocessableEntity, Message: "invalid size"})
	assert.Equal(t, output.ExitUserError, output.ClassifyError(err))

	var resp map[string]interface{}
	require.NoError(t, json.NewDecoder(stdout).Decode(&resp))
	data := resp["data"].(map[string]interface{})
	assert.Equal(t, reasonProvisionFailed, data["reason"])
	assert.Equal(t, float64(output.ExitUserError), data["exit_code"])
}

type timeoutErr struct{}

func (timeoutErr) Error() string   { return "i/o timeout" }
func (timeoutErr) Timeout() bool   { return true }
func (timeoutErr) Temporary() bool { return true }

var _ net.Error = timeoutErr{}

// A provision_failed reported for a network failure from server creation keeps
// the network code (4), in the process exit and the --json exit_code.
func TestWriteLaunchError_ProvisionFailedNetworkErrorKeepsItsCode(t *testing.T) {
	env, stdout, _ := testLaunchEnvelopeJSON()
	netErr := &url.Error{Op: "Post", URL: "https://example.test", Err: timeoutErr{}}
	err := writeLaunchError(env, launchConfig{}, reasonProvisionFailed, netErr)
	assert.Equal(t, output.ExitNetworkError, output.ClassifyError(err))

	var resp map[string]interface{}
	require.NoError(t, json.NewDecoder(stdout).Decode(&resp))
	data := resp["data"].(map[string]interface{})
	assert.Equal(t, reasonProvisionFailed, data["reason"])
	assert.Equal(t, float64(output.ExitNetworkError), data["exit_code"])
}

// repo_unreachable is a user error when the repository was refused, but a
// network or server failure behind it keeps its own code.
func TestLaunchRepoFailure_KeepsCauseExitCode(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want int
	}{
		{"rejected repository", &sdk.APIError{StatusCode: http.StatusUnprocessableEntity, Message: "invalid url"}, output.ExitUserError},
		{"network timeout", &url.Error{Op: "Post", URL: "https://example.test", Err: timeoutErr{}}, output.ExitNetworkError},
		{"server error", &sdk.APIError{StatusCode: http.StatusInternalServerError}, output.ExitInternalError},
		{"forbidden", &sdk.APIError{StatusCode: http.StatusForbidden}, output.ExitAuthError},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := launchRepoFailure(tc.err)
			var le *launchError
			require.ErrorAs(t, err, &le)
			assert.Equal(t, reasonRepoUnreachable, le.Reason)
			assert.Equal(t, tc.want, output.ClassifyError(err))
		})
	}
}

// A poll that is refused (401/403/404) keeps the code of that refusal; a
// resource that lands in "error" is a server-side failure (2).
func TestPollProvisioningState_RefusedPollKeepsStatusExitCode(t *testing.T) {
	cases := []struct {
		status int
		want   int
	}{
		{http.StatusUnauthorized, output.ExitAuthError},
		{http.StatusForbidden, output.ExitAuthError},
		{http.StatusNotFound, output.ExitNotFoundError},
	}
	for _, tc := range cases {
		t.Run(http.StatusText(tc.status), func(t *testing.T) {
			shrinkPollBackoff(t)
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(tc.status)
				_, _ = w.Write([]byte(`{"error":"refused"}`))
			}))
			t.Cleanup(srv.Close)

			env, _, _ := testLaunchEnvelope()
			pending := &sdk.Server{Identifier: "srv-1", ProtocolType: "static_hosting", StaticHosting: &sdk.StaticHostingInfo{Status: "provisioning"}}
			_, err := pollProvisioningState(t.Context(), env, newTestClient(t, srv), "my-app", pending, 10*time.Second)
			var le *launchError
			require.ErrorAs(t, err, &le)
			assert.Equal(t, reasonProvisionFailed, le.Reason)
			assert.Equal(t, tc.want, output.ClassifyError(err))
		})
	}

	shrinkPollBackoff(t)
	srv, _ := pollStateServer(t, []string{"error"})
	env, _, _ := testLaunchEnvelope()
	pending := &sdk.Server{Identifier: "srv-1", ProtocolType: "static_hosting", StaticHosting: &sdk.StaticHostingInfo{Status: "provisioning"}}
	_, err := pollProvisioningState(t.Context(), env, newSpecValidatingClient(t, srv), "my-app", pending, 10*time.Second)
	assert.Equal(t, output.ExitInternalError, output.ClassifyError(err))
}
