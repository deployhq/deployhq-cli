package commands

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAccountCommand_Registered(t *testing.T) {
	cmd := NewRootCmd("test")
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetArgs([]string{"account", "--help"})

	require.NoError(t, cmd.Execute())
	out := stdout.String()
	assert.Contains(t, out, "get")
	assert.Contains(t, out, "update")
	assert.Contains(t, out, "billing")
}

func TestAccountUpdateCommand_Help(t *testing.T) {
	cmd := NewRootCmd("test")
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetArgs([]string{"account", "update", "--help"})

	require.NoError(t, cmd.Execute())
	out := stdout.String()
	assert.Contains(t, out, "--name")
	assert.Contains(t, out, "--ip-restricted")
	assert.Contains(t, out, "--cname")
}

func TestAgentMetadata_AccountGet(t *testing.T) {
	m := lookupAgentMetadata("dhq account get")
	assert.True(t, m.Idempotent)
	assert.True(t, m.SupportsJSON)
	assert.True(t, m.SafeForAutomation)
	assert.False(t, m.Destructive)
	assert.Contains(t, m.ResourceTypes, "account")
}

func TestAgentMetadata_AccountUpdate(t *testing.T) {
	m := lookupAgentMetadata("dhq account update")
	assert.True(t, m.Idempotent)
	assert.True(t, m.SupportsJSON)
	assert.True(t, m.SafeForAutomation)
	assert.False(t, m.Destructive)
	assert.Contains(t, m.ResourceTypes, "account")
}

func TestAgentMetadata_AccountBilling(t *testing.T) {
	m := lookupAgentMetadata("dhq account billing")
	assert.True(t, m.Idempotent)
	assert.True(t, m.SupportsJSON)
	assert.True(t, m.SafeForAutomation)
	assert.Contains(t, m.ResourceTypes, "account")
}
