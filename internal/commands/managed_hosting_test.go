package commands

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestManagedHostingCmd_Structure(t *testing.T) {
	cmd := newManagedHostingCmd()
	assert.Equal(t, "managed-hosting", cmd.Name())

	want := map[string]bool{"regions": false, "sizes": false}
	for _, sub := range cmd.Commands() {
		if _, ok := want[sub.Name()]; ok {
			want[sub.Name()] = true
		}
	}
	for name, found := range want {
		assert.Truef(t, found, "missing subcommand %q", name)
	}
}

func TestManagedHostingCmd_Help(t *testing.T) {
	cmd := newManagedHostingCmd()
	cmd.SetArgs([]string{"--help"})
	require.NoError(t, cmd.Execute())
}
