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

func TestParseJSONFlag(t *testing.T) {
	cases := []struct {
		raw        string
		wantJSON   bool
		wantFields []string
	}{
		// Not set → auto behaviour
		{"", false, nil},

		// Truthy → force JSON, no field selection
		{"true", true, nil},
		{"TRUE", true, nil},
		{"1", true, nil},
		{"yes", true, nil},
		{"on", true, nil},

		// Falsy → explicit opt-out, preserves auto behaviour (the bug fix:
		// previously --json=false silently filtered to {} because "false"
		// was treated as a field name).
		{"false", false, nil},
		{"False", false, nil},
		{"0", false, nil},
		{"no", false, nil},
		{"off", false, nil},

		// Field selection → force JSON, pick fields
		{"name", true, []string{"name"}},
		{"name,permalink", true, []string{"name", "permalink"}},
		{"name,permalink,zone", true, []string{"name", "permalink", "zone"}},

		// Whitespace tolerance — quoted args can pick up stray spaces
		{"name, permalink", true, []string{"name", "permalink"}},
		{" name , permalink ", true, []string{"name", "permalink"}},
		{"name,,permalink", true, []string{"name", "permalink"}},
		{"name,permalink,", true, []string{"name", "permalink"}},

		// Whitespace-only / commas-only → treat as bare --json
		{"   ", true, nil},
		{",,,", true, nil},
	}
	for _, tc := range cases {
		t.Run("raw="+tc.raw, func(t *testing.T) {
			gotJSON, gotFields := parseJSONFlag(tc.raw)
			assert.Equal(t, tc.wantJSON, gotJSON, "JSON mode mismatch")
			assert.Equal(t, tc.wantFields, gotFields, "fields mismatch")
		})
	}
}

func TestGlobalOutputFlags_Registered(t *testing.T) {
	cmd := NewRootCmd("test")
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetArgs([]string{"projects", "list", "--help"})

	err := cmd.Execute()
	require.NoError(t, err)

	out := stdout.String()
	assert.Contains(t, out, "--table", "table flag should be available globally")
	assert.Contains(t, out, "--quiet", "quiet flag should be available globally")
	assert.Contains(t, out, "-q,", "quiet should have -q shorthand")
	assert.Contains(t, out, "--json=false to opt out", "json help should mention opt-out")
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
