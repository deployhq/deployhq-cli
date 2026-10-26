package commands

import (
	"bytes"
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/deployhq/deployhq-cli/internal/output"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// serverGroup returns the decoded `server_group` object from the captured body.
func (c *capturedRequest) serverGroup(t *testing.T) map[string]any {
	t.Helper()
	c.mu.Lock()
	defer c.mu.Unlock()
	require.Equal(t, 1, c.count, "expected exactly one DeployHQ API request")
	sg, ok := c.body["server_group"].(map[string]any)
	require.True(t, ok, "request body must wrap a server_group object, got: %v", c.body)
	return sg
}

// ── --transfer-order on the wire ──────────────────────────────────────────────
//
// The backend validates transfer_order against sequential/parallel. The CLI
// sends it only when the operator names one, so an omitted flag leaves the
// choice to the backend's default rather than to a value the CLI invented.

func TestServerGroupsCreate_SendsTransferOrderWhenGiven(t *testing.T) {
	for _, order := range []string{"parallel", "sequential"} {
		t.Run(order, func(t *testing.T) {
			withResolvableContext(t)
			cap := captureRequest(t)

			err := runServersCmd(t, "server-groups", "create",
				"--name", "web", "--transfer-order", order)
			require.NoError(t, err)

			sg := cap.serverGroup(t)
			assert.Equal(t, http.MethodPost, cap.method)
			assert.Equal(t, "/projects/my-app/server_groups", cap.path)
			assert.Equal(t, "web", sg["name"])
			assert.Equal(t, order, sg["transfer_order"])
		})
	}
}

func TestServerGroupsCreate_OmitsTransferOrderWhenNotGiven(t *testing.T) {
	withResolvableContext(t)
	cap := captureRequest(t)

	err := runServersCmd(t, "server-groups", "create", "--name", "web")
	require.NoError(t, err)

	sg := cap.serverGroup(t)
	assert.Equal(t, "web", sg["name"])
	assert.NotContains(t, sg, "transfer_order",
		"an omitted --transfer-order must not be sent, so the backend default applies")
}

func TestServerGroupsUpdate_SendsTransferOrderWhenGiven(t *testing.T) {
	withResolvableContext(t)
	cap := captureRequest(t)

	err := runServersCmd(t, "server-groups", "update", "sg-1", "--transfer-order", "parallel")
	require.NoError(t, err)

	sg := cap.serverGroup(t)
	assert.Equal(t, http.MethodPut, cap.method)
	assert.Equal(t, "/projects/my-app/server_groups/sg-1", cap.path)
	assert.Equal(t, "parallel", sg["transfer_order"])
	assert.NotContains(t, sg, "name", "an update must not send a name the operator did not give")
}

func TestServerGroupsUpdate_OmitsTransferOrderWhenNotGiven(t *testing.T) {
	withResolvableContext(t)
	cap := captureRequest(t)

	err := runServersCmd(t, "server-groups", "update", "sg-1", "--name", "Web EU")
	require.NoError(t, err)

	sg := cap.serverGroup(t)
	assert.Equal(t, "Web EU", sg["name"])
	assert.NotContains(t, sg, "transfer_order",
		"an update must not disturb the transfer order when the flag was omitted")
}

// ── local validation: fails before any network access ─────────────────────────

func TestServerGroupsCreate_RejectsUnknownTransferOrder_NoHTTP(t *testing.T) {
	for _, tc := range []struct{ name, order string }{
		{"unknown", "serial"},
		{"wrong case", "Parallel"}, // the backend's inclusion check is case-sensitive
		{"explicit empty", ""},     // a mistake, not a request for the default
	} {
		t.Run(tc.name, func(t *testing.T) {
			withResolvableContext(t)
			called := blockNetwork(t)

			err := runServersCmd(t, "server-groups", "create",
				"--name", "web", "--transfer-order", tc.order)

			require.Error(t, err)
			var userErr *output.UserError
			require.True(t, errors.As(err, &userErr), "must be a user error, got %T: %v", err, err)
			assert.Contains(t, userErr.Message, "Invalid --transfer-order")
			assert.Contains(t, userErr.Hint, "sequential")
			assert.Contains(t, userErr.Hint, "parallel")
			assert.False(t, *called, "validation must fail before any HTTP request")
		})
	}
}

func TestServerGroupsUpdate_RejectsUnknownTransferOrder_NoHTTP(t *testing.T) {
	withResolvableContext(t)
	called := blockNetwork(t)

	err := runServersCmd(t, "server-groups", "update", "sg-1", "--transfer-order", "serial")

	require.Error(t, err)
	var userErr *output.UserError
	require.True(t, errors.As(err, &userErr), "must be a user error, got %T: %v", err, err)
	assert.Contains(t, userErr.Message, "Invalid --transfer-order")
	assert.False(t, *called, "validation must fail before any HTTP request")
}

// A missing name is still reported first, in the same style as before.
func TestServerGroupsCreate_MissingNameStillRejected_NoHTTP(t *testing.T) {
	withResolvableContext(t)
	called := blockNetwork(t)

	err := runServersCmd(t, "server-groups", "create", "--transfer-order", "parallel")

	require.Error(t, err)
	var userErr *output.UserError
	require.True(t, errors.As(err, &userErr), "must be a user error, got %T: %v", err, err)
	assert.Equal(t, "Name is required", userErr.Message)
	assert.False(t, *called, "validation must fail before any HTTP request")
}

// ── help ──────────────────────────────────────────────────────────────────────

// The project and template server-group commands take the same flag and must
// describe it the same way: both name the two accepted values.
func TestServerGroupsHelp_ListsTransferOrderValues(t *testing.T) {
	for _, path := range [][]string{
		{"server-groups", "create"},
		{"server-groups", "update"},
		{"templates", "server-groups", "create"},
		{"templates", "server-groups", "update"},
	} {
		t.Run(strings.Join(path, " "), func(t *testing.T) {
			root := NewRootCmd("test")
			var stdout bytes.Buffer
			root.SetOut(&stdout)
			root.SetErr(&stdout)
			root.SetArgs(append(append([]string{}, path...), "--help"))
			require.NoError(t, root.Execute())

			help := stdout.String()
			assert.Contains(t, help, "--transfer-order")
			assert.Contains(t, help, "sequential")
			assert.Contains(t, help, "parallel")
		})
	}
}
