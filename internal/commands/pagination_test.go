package commands

import (
	"testing"

	"github.com/deployhq/deployhq-cli/pkg/sdk"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestListOptsFromFlags(t *testing.T) {
	assert.Nil(t, listOptsFromFlags(0, 0))
	assert.Equal(t, &sdk.ListOptions{Page: 2}, listOptsFromFlags(2, 0))
	assert.Equal(t, &sdk.ListOptions{PerPage: 10}, listOptsFromFlags(0, 10))
	assert.Equal(t, &sdk.ListOptions{Page: 2, PerPage: 10}, listOptsFromFlags(2, 10))
}

func TestDeploymentsListPaginationFlags(t *testing.T) {
	cmd := NewRootCmd("test")
	cmd.SetArgs([]string{"deployments", "list", "--help"})
	err := cmd.Execute()
	assert.NoError(t, err)

	// Find the list subcommand and verify flags exist
	deploymentsCmd, _, err2 := cmd.Find([]string{"deployments", "list"})
	require.NoError(t, err2)
	assert.NotNil(t, deploymentsCmd.Flags().Lookup("page"))
	assert.NotNil(t, deploymentsCmd.Flags().Lookup("per-page"))
}

func TestServersListPaginationFlags(t *testing.T) {
	cmd := NewRootCmd("test")
	serversCmd, _, err := cmd.Find([]string{"servers", "list"})
	require.NoError(t, err)
	assert.NotNil(t, serversCmd.Flags().Lookup("page"))
	assert.NotNil(t, serversCmd.Flags().Lookup("per-page"))
}

func TestServerGroupsListPaginationFlags(t *testing.T) {
	cmd := NewRootCmd("test")
	sgCmd, _, err := cmd.Find([]string{"server-groups", "list"})
	require.NoError(t, err)
	assert.NotNil(t, sgCmd.Flags().Lookup("page"))
	assert.NotNil(t, sgCmd.Flags().Lookup("per-page"))
}
