package commands

import (
	"bytes"
	"errors"
	"testing"

	"github.com/deployhq/deployhq-cli/internal/output"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type capturedInstallCmd struct {
	name string
	args []string
}

func stubRunInstallCmd(t *testing.T, result error) *[]capturedInstallCmd {
	t.Helper()
	calls := &[]capturedInstallCmd{}
	orig := runInstallCmd
	runInstallCmd = func(name string, args []string) error {
		*calls = append(*calls, capturedInstallCmd{name: name, args: args})
		return result
	}
	t.Cleanup(func() { runInstallCmd = orig })
	return calls
}

func hasBatchMode(args []string) bool {
	for i := 0; i+1 < len(args); i++ {
		if args[i] == "-o" && args[i+1] == "BatchMode=yes" {
			return true
		}
	}
	return false
}

func TestInstallSSHKey_NonInteractiveNeverPrompts(t *testing.T) {
	calls := stubRunInstallCmd(t, errors.New("exit status 255"))
	env := &output.Envelope{Stderr: &bytes.Buffer{}, NonInteractive: true}

	err := installSSHKey(env, "h", 22, "deploy", "ssh-ed25519 AAAA k")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "non-interactive mode")

	require.NotEmpty(t, *calls)
	for _, c := range *calls {
		assert.True(t, hasBatchMode(c.args), "%s must run with BatchMode=yes: %v", c.name, c.args)
	}
}

func TestInstallSSHKey_InteractiveAllowsPasswordPrompt(t *testing.T) {
	calls := stubRunInstallCmd(t, nil)
	env := &output.Envelope{Stderr: &bytes.Buffer{}, IsTTY: true}

	require.NoError(t, installSSHKey(env, "h", 2222, "deploy", "ssh-ed25519 AAAA k"))
	require.Len(t, *calls, 1)
	assert.False(t, hasBatchMode((*calls)[0].args))
	assert.Contains(t, (*calls)[0].args, "2222")
}

func TestSSHInstallArgs_BatchMode(t *testing.T) {
	assert.True(t, hasBatchMode(sshCopyIDArgs("/tmp/k.pub", "h", "22", "u", true)))
	assert.True(t, hasBatchMode(sshAppendArgs("h", "22", "u", "k", true)))
	assert.False(t, hasBatchMode(sshCopyIDArgs("/tmp/k.pub", "h", "22", "u", false)))
	assert.False(t, hasBatchMode(sshAppendArgs("h", "22", "u", "k", false)))
	// Destination stays last for ssh-copy-id; the script stays last for ssh.
	a := sshCopyIDArgs("/tmp/k.pub", "h", "22", "u", true)
	assert.Equal(t, "u@h", a[len(a)-1])
	b := sshAppendArgs("h", "22", "u", "k", true)
	assert.Equal(t, "u@h", b[len(b)-2])
}

func TestSSHCopyIDArgs_ForceModeForPublicKeyOnly(t *testing.T) {
	assert.Equal(t,
		[]string{"-f", "-i", "/tmp/k.pub", "-p", "22", "-o", "StrictHostKeyChecking=accept-new", "u@h"},
		sshCopyIDArgs("/tmp/k.pub", "h", "22", "u", false))
	assert.Equal(t,
		[]string{"-f", "-i", "/tmp/k.pub", "-p", "22", "-o", "StrictHostKeyChecking=accept-new",
			"-o", "BatchMode=yes", "-o", "ConnectTimeout=15", "u@h"},
		sshCopyIDArgs("/tmp/k.pub", "h", "22", "u", true))
}
