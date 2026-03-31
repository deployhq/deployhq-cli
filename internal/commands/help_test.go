package commands

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHelpAgent_RootCommand(t *testing.T) {
	cmd := NewRootCmd("test")
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetArgs([]string{"--help", "--agent"})

	err := cmd.Execute()
	require.NoError(t, err)

	var schema AgentHelpSchema
	require.NoError(t, json.Unmarshal(stdout.Bytes(), &schema))

	assert.Equal(t, "dhq", schema.Name)
	assert.NotEmpty(t, schema.Subcommands)
	assert.NotEmpty(t, schema.Flags)

	// Should have global flags
	flagNames := make([]string, 0)
	for _, f := range schema.Flags {
		flagNames = append(flagNames, f.Name)
	}
	assert.Contains(t, flagNames, "account")
	assert.Contains(t, flagNames, "project")
	assert.Contains(t, flagNames, "json")
}

func TestHelpAgent_Subcommand(t *testing.T) {
	cmd := NewRootCmd("test")
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetArgs([]string{"projects", "--help", "--agent"})

	err := cmd.Execute()
	require.NoError(t, err)

	var schema AgentHelpSchema
	require.NoError(t, json.Unmarshal(stdout.Bytes(), &schema))

	assert.Equal(t, "projects", schema.Name)
	assert.Equal(t, "dhq projects", schema.FullCommand)
	assert.NotEmpty(t, schema.Subcommands)

	// Check subcommands include list, show, create
	subNames := make([]string, 0)
	for _, s := range schema.Subcommands {
		subNames = append(subNames, s.Name)
	}
	assert.Contains(t, subNames, "list")
	assert.Contains(t, subNames, "show")
	assert.Contains(t, subNames, "create")
}

func TestHelpAgent_LeafCommand(t *testing.T) {
	cmd := NewRootCmd("test")
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetArgs([]string{"deploy", "--help", "--agent"})

	err := cmd.Execute()
	require.NoError(t, err)

	var schema AgentHelpSchema
	require.NoError(t, json.Unmarshal(stdout.Bytes(), &schema))

	assert.Equal(t, "deploy", schema.Name)

	// Should have local flags like --branch, --server
	flagNames := make([]string, 0)
	for _, f := range schema.Flags {
		flagNames = append(flagNames, f.Name)
	}
	assert.Contains(t, flagNames, "branch")
	assert.Contains(t, flagNames, "server")
}

func TestNewCommands_Registered(t *testing.T) {
	cmd := NewRootCmd("test")
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetArgs([]string{"--help"})

	err := cmd.Execute()
	require.NoError(t, err)

	out := stdout.String()
	assert.Contains(t, out, "auto-deploys")
	assert.Contains(t, out, "scheduled-deploys")
	assert.Contains(t, out, "build-configs")
	assert.Contains(t, out, "mcp")
}
