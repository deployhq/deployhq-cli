package commands

import (
	"bytes"
	"testing"

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
