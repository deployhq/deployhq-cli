package commands

import (
	"bytes"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/deployhq/deployhq-cli/internal/output"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProfileCommand_Registered(t *testing.T) {
	cmd := NewRootCmd("test")
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetArgs([]string{"profile", "--help"})

	require.NoError(t, cmd.Execute())
	out := stdout.String()
	assert.Contains(t, out, "get")
	assert.Contains(t, out, "update")
}

func TestProfileUpdateCommand_Help(t *testing.T) {
	cmd := NewRootCmd("test")
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetArgs([]string{"profile", "update", "--help"})

	require.NoError(t, cmd.Execute())
	out := stdout.String()
	assert.Contains(t, out, "--first-name")
	assert.Contains(t, out, "--alphabetic-sort")
	assert.Contains(t, out, "--promotions-enabled")
}

// --email no longer changes the address. It must fail before any request is made: if it fell
// through to the global --email auth flag, the update would run as a different user.
func TestProfileUpdateCommand_EmailIsRefusedWithoutCallingTheAPI(t *testing.T) {
	withResolvableContext(t)
	var profileCalls int
	orig := http.DefaultTransport
	http.DefaultTransport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if strings.HasSuffix(r.URL.Path, "/profile") {
			profileCalls++
		}
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`{}`)),
			Header: http.Header{"Content-Type": []string{"application/json"}}, Request: r}, nil
	})
	t.Cleanup(func() { http.DefaultTransport = orig })

	_, err := runCmdCapturingStdout(t, "profile", "update", "--email", "attacker@example.com", "--first-name", "Z")

	var userErr *output.UserError
	require.ErrorAs(t, err, &userErr)
	assert.Contains(t, userErr.Hint, "identity.deployhq.com")
	assert.Zero(t, profileCalls)
}

func TestAgentMetadata_ProfileGet(t *testing.T) {
	m := lookupAgentMetadata("dhq profile get")
	assert.True(t, m.Idempotent)
	assert.True(t, m.SupportsJSON)
	assert.True(t, m.SafeForAutomation)
	assert.False(t, m.Destructive)
	assert.Contains(t, m.ResourceTypes, "profile")
}

func TestAgentMetadata_ProfileUpdate(t *testing.T) {
	m := lookupAgentMetadata("dhq profile update")
	assert.True(t, m.Idempotent)
	assert.True(t, m.SupportsJSON)
	assert.True(t, m.SafeForAutomation)
	assert.False(t, m.Destructive)
	assert.Contains(t, m.ResourceTypes, "profile")
}
