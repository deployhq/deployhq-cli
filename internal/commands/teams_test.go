package commands

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTeamsCommand_Registered(t *testing.T) {
	cmd := NewRootCmd("test")

	var found bool
	for _, child := range cmd.Commands() {
		if child.Name() == "teams" {
			found = true
			break
		}
	}
	assert.True(t, found, "teams command must be registered on root")
}

func TestTeamsCommand_Help(t *testing.T) {
	cmd := NewRootCmd("test")
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetArgs([]string{"teams", "--help"})

	err := cmd.Execute()
	require.NoError(t, err)

	out := stdout.String()
	assert.Contains(t, out, "list")
	assert.Contains(t, out, "show")
	assert.Contains(t, out, "create")
	assert.Contains(t, out, "update")
	assert.Contains(t, out, "delete")
}

func TestTeamsCreate_Help(t *testing.T) {
	cmd := NewRootCmd("test")
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetArgs([]string{"teams", "create", "--help"})

	err := cmd.Execute()
	require.NoError(t, err)

	out := stdout.String()
	assert.Contains(t, out, "--admin")
	assert.Contains(t, out, "--can-manage-users")
	assert.Contains(t, out, "--can-manage-billing")
	assert.Contains(t, out, "--can-manage-agents")
	assert.Contains(t, out, "--can-create-projects")
	assert.Contains(t, out, "--all-projects")
	assert.Contains(t, out, "--user-ids")
}

func TestAgentMetadata_TeamsList(t *testing.T) {
	m := lookupAgentMetadata("dhq teams list")
	assert.True(t, m.Idempotent)
	assert.True(t, m.SupportsJSON)
	assert.True(t, m.SafeForAutomation)
	assert.False(t, m.Destructive)
	assert.Contains(t, m.ResourceTypes, "team")
}

func TestAgentMetadata_TeamsShow(t *testing.T) {
	m := lookupAgentMetadata("dhq teams show")
	assert.True(t, m.Idempotent)
	assert.True(t, m.SupportsJSON)
	assert.True(t, m.SafeForAutomation)
	assert.Contains(t, m.ResourceTypes, "team")
}

func TestAgentMetadata_TeamsCreate(t *testing.T) {
	m := lookupAgentMetadata("dhq teams create")
	assert.False(t, m.Idempotent, "create is not idempotent")
	assert.True(t, m.SupportsJSON)
	assert.True(t, m.SafeForAutomation)
	assert.False(t, m.Destructive)
	assert.Contains(t, m.ResourceTypes, "team")
}

func TestAgentMetadata_TeamsUpdate(t *testing.T) {
	m := lookupAgentMetadata("dhq teams update")
	assert.True(t, m.Idempotent, "update is idempotent")
	assert.True(t, m.SupportsJSON)
	assert.True(t, m.SafeForAutomation)
	assert.Contains(t, m.ResourceTypes, "team")
}

func TestAgentMetadata_TeamsDelete(t *testing.T) {
	m := lookupAgentMetadata("dhq teams delete")
	assert.True(t, m.Destructive)
	assert.True(t, m.RequiresConfirmation)
	assert.True(t, m.SupportsJSON)
	assert.True(t, m.SafeForAutomation)
	assert.Contains(t, m.ResourceTypes, "team")
}
