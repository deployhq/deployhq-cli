package commands

import (
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/deployhq/deployhq-cli/internal/output"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// installServerCreateRefusal answers server creation with status and body.
func installServerCreateRefusal(t *testing.T, status int, body string) {
	t.Helper()
	orig := http.DefaultTransport
	http.DefaultTransport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		respondStatus, respondBody := http.StatusOK, `{}`
		if r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/projects/my-app/servers") {
			respondStatus, respondBody = status, body
		}
		return &http.Response{
			StatusCode: respondStatus,
			Body:       io.NopCloser(strings.NewReader(respondBody)),
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Request:    r,
		}, nil
	})
	t.Cleanup(func() { http.DefaultTransport = orig })
}

// notEnrolledBody is DeployHQ's refusal of a beta server type on an account
// that is not enrolled in the beta.
const notEnrolledBody = `{"type":["is not available for your account"]}`

func TestServersCreate_BetaTypeNotEnrolledPointsAtBetaEnroll(t *testing.T) {
	for _, tc := range []struct {
		protocol string
		args     []string
	}{
		{"static_hosting", []string{"--subdomain", "my-site"}},
		{"managed_vps", []string{"--region", "lon1", "--size", "s-1vcpu-1gb", "--accept-cost"}},
	} {
		t.Run(tc.protocol, func(t *testing.T) {
			withResolvableContext(t)
			installServerCreateRefusal(t, http.StatusUnprocessableEntity, notEnrolledBody)

			args := append([]string{"servers", "create", "--name", "site", "--protocol-type", tc.protocol}, tc.args...)
			err := runServersCmd(t, args...)

			var uerr *output.UserError
			require.True(t, errors.As(err, &uerr), "got %T: %v", err, err)
			assert.Equal(t, output.ExitUserError, output.ClassifyError(err))
			assert.Contains(t, uerr.Message, "not available for your account", "the server's refusal is kept")
			assert.Contains(t, uerr.Hint, "dhq beta enroll --protocol "+tc.protocol)
			assert.Contains(t, uerr.Hint, "account admin")

			data := output.ErrorResponseFromErr(err).Data.(output.ErrorData)
			assert.Equal(t, []output.Breadcrumb{{Action: "enroll_in_beta", Cmd: "dhq beta enroll --protocol " + tc.protocol}}, data.Recovery)
		})
	}
}

func TestServersCreate_OtherValidationErrorsUnchanged(t *testing.T) {
	withResolvableContext(t)
	installServerCreateRefusal(t, http.StatusUnprocessableEntity, `{"subdomain":["has already been taken"]}`)

	err := runServersCmd(t, "servers", "create", "--name", "site", "--protocol-type", "static_hosting", "--subdomain", "taken")
	require.Error(t, err)
	var uerr *output.UserError
	assert.False(t, errors.As(err, &uerr), "got %v", err)
	assert.NotContains(t, err.Error(), "beta enroll")
}
