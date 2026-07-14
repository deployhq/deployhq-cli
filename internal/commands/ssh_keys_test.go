package commands

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The ssh-keys command must expose the download subcommand and its --output flag.
func TestSSHKeysDownloadCommand_Help(t *testing.T) {
	cmd := NewRootCmd("test")
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetArgs([]string{"ssh-keys", "--help"})
	require.NoError(t, cmd.Execute())
	assert.Contains(t, stdout.String(), "download")

	var dl bytes.Buffer
	cmd2 := NewRootCmd("test")
	cmd2.SetOut(&dl)
	cmd2.SetArgs([]string{"ssh-keys", "download", "--help"})
	require.NoError(t, cmd2.Execute())
	assert.Contains(t, dl.String(), "--output")
}

func TestAgentMetadata_SSHKeysDownload(t *testing.T) {
	m := lookupAgentMetadata("dhq ssh-keys download")
	assert.True(t, m.Idempotent, "reading a key is retry-safe")
	assert.True(t, m.SupportsJSON)
	assert.False(t, m.Destructive, "downloading does not mutate the key")
	assert.True(t, m.RequiresConfirmation, "emits sensitive private key material")
	assert.False(t, m.SafeForAutomation, "key material should not be printed unattended")
	assert.Contains(t, m.ResourceTypes, "ssh_key")
}
