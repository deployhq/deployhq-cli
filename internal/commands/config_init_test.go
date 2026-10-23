package commands

import (
	"io"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// runConfigInit runs 'dhq <args>' in a fresh directory with an isolated HOME
// and returns the resulting .deployhq.toml contents.
func runConfigInit(t *testing.T, args ...string) (string, error) {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	for _, k := range []string{"DEPLOYHQ_ACCOUNT", "DEPLOYHQ_EMAIL", "DEPLOYHQ_API_KEY", "DEPLOYHQ_PROJECT", "DEPLOYHQ_HOST"} {
		t.Setenv(k, "")
	}
	t.Chdir(t.TempDir())

	cmd := NewRootCmd("dev")
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	cmd.SetArgs(args)
	err := cmd.Execute()
	data, _ := os.ReadFile(".deployhq.toml")
	return string(data), err
}

// Values passed as --project / --account must land in the new file, not an
// empty project entry.
func TestConfigInit_WritesGivenFlags(t *testing.T) {
	data, err := runConfigInit(t, "config", "init", "--non-interactive", "-p", "s3-private", "--account", "test-account")
	require.NoError(t, err)
	assert.Contains(t, data, "project = 's3-private'")
	assert.Contains(t, data, "account = 'test-account'")
}

// The API key must never be written to a project config file.
func TestConfigInit_NeverWritesAPIKey(t *testing.T) {
	data, err := runConfigInit(t, "config", "init", "--non-interactive", "-p", "my-app", "--api-key", "secret-key-value")
	require.NoError(t, err)
	assert.Contains(t, data, "project = 'my-app'")
	assert.NotContains(t, data, "secret-key-value")
	assert.NotContains(t, data, "api_key")
}

// Without flags the previous behaviour is kept: an empty project entry.
func TestConfigInit_NoFlagsCreatesEmptyProject(t *testing.T) {
	data, err := runConfigInit(t, "config", "init", "--non-interactive")
	require.NoError(t, err)
	assert.Contains(t, data, "project = ''")
}
