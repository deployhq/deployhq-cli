package commands

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The folders command must be registered on the root command and expose its
// full subcommand tree.
func TestFoldersCommand_Registered(t *testing.T) {
	cmd := NewRootCmd("test")
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetArgs([]string{"folders", "--help"})

	require.NoError(t, cmd.Execute())
	out := stdout.String()
	assert.Contains(t, out, "list")
	assert.Contains(t, out, "create")
	assert.Contains(t, out, "update")
	assert.Contains(t, out, "delete")
}

// update requires --name; its help must advertise the flag.
func TestFoldersUpdateCommand_Help(t *testing.T) {
	cmd := NewRootCmd("test")
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetArgs([]string{"folders", "update", "--help"})

	require.NoError(t, cmd.Execute())
	assert.Contains(t, stdout.String(), "--name")
}

func TestAgentMetadata_FoldersList(t *testing.T) {
	m := lookupAgentMetadata("dhq folders list")
	assert.True(t, m.Idempotent)
	assert.True(t, m.SupportsJSON)
	assert.True(t, m.SafeForAutomation)
	assert.False(t, m.Destructive)
	assert.Contains(t, m.ResourceTypes, "folder")
}

func TestAgentMetadata_FoldersUpdate(t *testing.T) {
	m := lookupAgentMetadata("dhq folders update")
	assert.True(t, m.Idempotent, "renaming a known folder to a given name is retry-safe")
	assert.True(t, m.SupportsJSON)
	assert.True(t, m.SafeForAutomation)
	assert.False(t, m.Destructive)
	assert.Contains(t, m.ResourceTypes, "folder")
}

func TestAgentMetadata_FoldersDelete(t *testing.T) {
	m := lookupAgentMetadata("dhq folders delete")
	assert.True(t, m.Destructive)
	assert.True(t, m.RequiresConfirmation)
	assert.True(t, m.SupportsJSON, "delete emits a JSON envelope in --json mode")
	assert.True(t, m.SafeForAutomation, "deterministic; gated by Destructive/RequiresConfirmation")
	assert.Contains(t, m.ResourceTypes, "folder")
}
