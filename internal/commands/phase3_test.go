package commands

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCommandTree_AllRegistered(t *testing.T) {
	cmd := NewRootCmd("test")
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetArgs([]string{"--help"})

	err := cmd.Execute()
	require.NoError(t, err)

	out := stdout.String()
	// Resource commands
	assert.Contains(t, out, "projects")
	assert.Contains(t, out, "servers")
	assert.Contains(t, out, "server-groups")
	assert.Contains(t, out, "deployments")
	assert.Contains(t, out, "repos")

	// Shortcuts
	assert.Contains(t, out, "deploy")
	assert.Contains(t, out, "rollback")

	// Escape hatch
	assert.Contains(t, out, "api")

	// Meta
	assert.Contains(t, out, "doctor")
}

func TestProjectsSubcommands(t *testing.T) {
	cmd := NewRootCmd("test")
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetArgs([]string{"projects", "--help"})

	err := cmd.Execute()
	require.NoError(t, err)

	out := stdout.String()
	for _, sub := range []string{"list", "show", "create", "update", "delete", "star", "insights"} {
		assert.Contains(t, out, sub, "projects should have %s subcommand", sub)
	}
}

func TestServersSubcommands(t *testing.T) {
	cmd := NewRootCmd("test")
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetArgs([]string{"servers", "--help"})

	err := cmd.Execute()
	require.NoError(t, err)

	out := stdout.String()
	for _, sub := range []string{"list", "show", "create", "update", "delete", "reset-host-key"} {
		assert.Contains(t, out, sub, "servers should have %s subcommand", sub)
	}
}

func TestDeploymentsSubcommands(t *testing.T) {
	cmd := NewRootCmd("test")
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetArgs([]string{"deployments", "--help"})

	err := cmd.Execute()
	require.NoError(t, err)

	out := stdout.String()
	for _, sub := range []string{"list", "show", "create", "abort", "rollback", "logs"} {
		assert.Contains(t, out, sub, "deployments should have %s subcommand", sub)
	}
}

func TestReposSubcommands(t *testing.T) {
	cmd := NewRootCmd("test")
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetArgs([]string{"repos", "--help"})

	err := cmd.Execute()
	require.NoError(t, err)

	out := stdout.String()
	for _, sub := range []string{"show", "update", "branches", "commits", "latest-revision"} {
		assert.Contains(t, out, sub, "repos should have %s subcommand", sub)
	}
}

func TestServerGroupsSubcommands(t *testing.T) {
	cmd := NewRootCmd("test")
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetArgs([]string{"server-groups", "--help"})

	err := cmd.Execute()
	require.NoError(t, err)

	out := stdout.String()
	for _, sub := range []string{"list", "show", "create", "update", "delete"} {
		assert.Contains(t, out, sub, "server-groups should have %s subcommand", sub)
	}
}

func TestAPICommand_Help(t *testing.T) {
	cmd := NewRootCmd("test")
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetArgs([]string{"api", "--help"})

	err := cmd.Execute()
	require.NoError(t, err)

	out := stdout.String()
	assert.Contains(t, out, "GET")
	assert.Contains(t, out, "POST")
	assert.Contains(t, out, "--body")
}

func TestDeployCommand_Help(t *testing.T) {
	cmd := NewRootCmd("test")
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetArgs([]string{"deploy", "--help"})

	err := cmd.Execute()
	require.NoError(t, err)

	out := stdout.String()
	assert.Contains(t, out, "--branch")
	assert.Contains(t, out, "--server")
	assert.Contains(t, out, "--revision")
}

func TestAliases(t *testing.T) {
	cmd := NewRootCmd("test")

	// Find the projects command and verify aliases
	for _, child := range cmd.Commands() {
		switch child.Use {
		case "projects":
			assert.Contains(t, child.Aliases, "proj")
		case "servers":
			assert.Contains(t, child.Aliases, "srv")
		case "server-groups":
			assert.Contains(t, child.Aliases, "sg")
		case "deployments":
			assert.Contains(t, child.Aliases, "dep")
		case "repos":
			assert.Contains(t, child.Aliases, "repo")
		}
	}
}
