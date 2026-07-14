package commands

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUsersCommand_Registered(t *testing.T) {
	cmd := NewRootCmd("test")
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetArgs([]string{"users", "--help"})

	require.NoError(t, cmd.Execute())
	out := stdout.String()
	assert.Contains(t, out, "list")
	assert.Contains(t, out, "show")
	assert.Contains(t, out, "create")
	assert.Contains(t, out, "update")
	assert.Contains(t, out, "delete")
	assert.Contains(t, out, "resend-invitation")
}

func TestUsersCreateCommand_Help(t *testing.T) {
	cmd := NewRootCmd("test")
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetArgs([]string{"users", "create", "--help"})

	require.NoError(t, cmd.Execute())
	out := stdout.String()
	assert.Contains(t, out, "--email")
	assert.Contains(t, out, "--first-name")
	assert.Contains(t, out, "--admin")
}

func TestAgentMetadata_UsersList(t *testing.T) {
	m := lookupAgentMetadata("dhq users list")
	assert.True(t, m.Idempotent)
	assert.True(t, m.SupportsJSON)
	assert.True(t, m.SafeForAutomation)
	assert.False(t, m.Destructive)
	assert.Contains(t, m.ResourceTypes, "user")
}

func TestAgentMetadata_UsersCreate(t *testing.T) {
	m := lookupAgentMetadata("dhq users create")
	assert.False(t, m.Idempotent)
	assert.True(t, m.SupportsJSON)
	assert.True(t, m.SafeForAutomation)
	assert.False(t, m.Destructive)
	assert.Contains(t, m.ResourceTypes, "user")
}

func TestAgentMetadata_UsersDelete(t *testing.T) {
	m := lookupAgentMetadata("dhq users delete")
	assert.True(t, m.Destructive)
	assert.True(t, m.RequiresConfirmation)
	assert.True(t, m.SupportsJSON)
	assert.True(t, m.SafeForAutomation)
	assert.Contains(t, m.ResourceTypes, "user")
}

func TestAgentMetadata_UsersResendInvitation(t *testing.T) {
	m := lookupAgentMetadata("dhq users resend-invitation")
	assert.False(t, m.Idempotent, "each call sends a fresh invitation email")
	assert.True(t, m.SupportsJSON)
	assert.True(t, m.SafeForAutomation)
	assert.Contains(t, m.ResourceTypes, "user")
}
