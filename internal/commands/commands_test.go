package commands

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRootCommand_Help(t *testing.T) {
	cmd := NewRootCmd("test-version")
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetArgs([]string{"--help"})

	err := cmd.Execute()
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "dhq")
	assert.Contains(t, stdout.String(), "auth")
	assert.Contains(t, stdout.String(), "config")
	assert.Contains(t, stdout.String(), "version")
}

func TestAuthCommand_Help(t *testing.T) {
	cmd := NewRootCmd("test")
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetArgs([]string{"auth", "--help"})

	err := cmd.Execute()
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "login")
	assert.Contains(t, stdout.String(), "logout")
	assert.Contains(t, stdout.String(), "status")
	assert.Contains(t, stdout.String(), "token")
}

func TestConfigCommand_Help(t *testing.T) {
	cmd := NewRootCmd("test")
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetArgs([]string{"config", "--help"})

	err := cmd.Execute()
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "show")
	assert.Contains(t, stdout.String(), "init")
	assert.Contains(t, stdout.String(), "set")
	assert.Contains(t, stdout.String(), "unset")
}

func TestConfigSet_InvalidKey(t *testing.T) {
	cmd := NewRootCmd("test")
	cmd.SetArgs([]string{"config", "set", "invalid_key", "value"})

	err := cmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Unknown config key")
}

func TestMaskAPIKey(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"", "(not set)"},
		{"short", "****"},
		{"1234567890abcdef", "1234****cdef"},
	}

	for _, tt := range tests {
		assert.Equal(t, tt.expected, maskAPIKey(tt.input))
	}
}

func TestIsValidKey(t *testing.T) {
	assert.True(t, isValidKey("account"))
	assert.True(t, isValidKey("email"))
	assert.True(t, isValidKey("api_key"))
	assert.True(t, isValidKey("project"))
	assert.True(t, isValidKey("format"))
	assert.False(t, isValidKey("unknown"))
	assert.False(t, isValidKey(""))
}

func TestGlobalFlags_Registered(t *testing.T) {
	cmd := NewRootCmd("test")
	flags := cmd.PersistentFlags()

	for _, name := range []string{"account", "email", "api-key", "project", "json", "cwd"} {
		assert.NotNil(t, flags.Lookup(name), "flag --%s should be registered", name)
	}
}
