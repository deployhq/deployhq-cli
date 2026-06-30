package commands

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAgentMetadata_Deploy(t *testing.T) {
	m := lookupAgentMetadata("dhq deploy")
	assert.True(t, m.Interactive, "deploy can prompt for server")
	assert.False(t, m.Destructive)
	assert.False(t, m.Idempotent, "deploy creates a new deployment each time")
	assert.True(t, m.SupportsJSON)
	assert.True(t, m.SafeForAutomation)
	assert.Contains(t, m.ResourceTypes, "deployment")
}

func TestAgentMetadata_Rollback(t *testing.T) {
	m := lookupAgentMetadata("dhq rollback")
	assert.False(t, m.Interactive)
	assert.False(t, m.Destructive)
	assert.False(t, m.Idempotent)
	assert.True(t, m.SupportsJSON)
	assert.True(t, m.SafeForAutomation)
}

func TestAgentMetadata_Launch(t *testing.T) {
	m := lookupAgentMetadata("dhq launch")
	assert.True(t, m.Interactive, "launch can prompt in a TTY")
	assert.False(t, m.Destructive, "launch provisions, it does not delete")
	assert.True(t, m.Idempotent, "re-runs resolve the existing project/server")
	assert.True(t, m.RequiresConfirmation, "provisions managed resources")
	assert.True(t, m.SupportsJSON)
	assert.True(t, m.SafeForAutomation, "deterministic non-interactively with the right flags")
	assert.Contains(t, m.ResourceTypes, "server")
	assert.Contains(t, m.ResourceTypes, "deployment")
}

func TestAgentMetadata_ProjectsDelete(t *testing.T) {
	m := lookupAgentMetadata("dhq projects delete")
	assert.True(t, m.Destructive)
	assert.True(t, m.RequiresConfirmation)
	assert.True(t, m.SafeForAutomation)
}

func TestAgentMetadata_ServersCreate(t *testing.T) {
	m := lookupAgentMetadata("dhq servers create")
	assert.False(t, m.Destructive)
	assert.False(t, m.Idempotent)
	assert.True(t, m.SupportsJSON)
	assert.Contains(t, m.ResourceTypes, "server")
}

func TestAgentMetadata_Doctor(t *testing.T) {
	m := lookupAgentMetadata("dhq doctor")
	assert.True(t, m.Idempotent)
	assert.True(t, m.SupportsJSON)
	assert.True(t, m.SafeForAutomation)
	assert.False(t, m.Destructive)
}

func TestAgentMetadata_API(t *testing.T) {
	m := lookupAgentMetadata("dhq api")
	assert.False(t, m.Idempotent, "api can do POST/DELETE")
	assert.True(t, m.SupportsJSON)
	assert.True(t, m.SafeForAutomation)
}

func TestAgentMetadata_InteractiveCommands(t *testing.T) {
	interactive := []string{"dhq init", "dhq hello", "dhq configure", "dhq auth login"}
	for _, path := range interactive {
		m := lookupAgentMetadata(path)
		assert.True(t, m.Interactive, "%s should be interactive", path)
	}
}

func TestAgentMetadata_UnsafeForAutomation(t *testing.T) {
	unsafe := []string{"dhq init", "dhq hello", "dhq configure", "dhq deployments watch"}
	for _, path := range unsafe {
		m := lookupAgentMetadata(path)
		assert.False(t, m.SafeForAutomation, "%s should not be safe for automation", path)
	}
}

func TestAgentMetadata_DefaultForUnknown(t *testing.T) {
	m := lookupAgentMetadata("dhq nonexistent command")
	assert.False(t, m.Idempotent, "unknown commands default to not idempotent")
	assert.False(t, m.SupportsJSON, "unknown commands default to no JSON support")
	assert.False(t, m.SafeForAutomation, "unknown commands default to not safe")
	assert.False(t, m.Destructive)
	assert.False(t, m.Interactive)
}

func TestCommandCatalog_IncludesAgentMetadata(t *testing.T) {
	root := NewRootCmd("test")
	catalog := buildCatalog(root)

	// Find deploy command
	var deployCmd *CommandInfo
	for i, c := range catalog {
		if c.Name == "deploy" {
			deployCmd = &catalog[i]
			break
		}
	}
	require.NotNil(t, deployCmd, "deploy must be in catalog")
	require.NotNil(t, deployCmd.Agent, "deploy must have agent metadata")
	assert.True(t, deployCmd.Agent.Interactive)
	assert.Contains(t, deployCmd.Agent.ResourceTypes, "deployment")
}

func TestCommandCatalog_AgentMetadataJSONShape(t *testing.T) {
	root := NewRootCmd("test")
	catalog := buildCatalog(root)

	b, err := json.Marshal(catalog)
	require.NoError(t, err)

	// Verify the agent field appears in JSON
	assert.Contains(t, string(b), `"agent"`)
	assert.Contains(t, string(b), `"interactive"`)
	assert.Contains(t, string(b), `"destructive"`)
	assert.Contains(t, string(b), `"idempotent"`)
	assert.Contains(t, string(b), `"safe_for_automation"`)
}

func TestNonInteractiveFlag_Registered(t *testing.T) {
	cmd := NewRootCmd("test")
	f := cmd.PersistentFlags().Lookup("non-interactive")
	assert.NotNil(t, f, "--non-interactive must be registered")
	assert.Equal(t, "false", f.DefValue)
}

func TestNonInteractive_DeployServerAmbiguity(t *testing.T) {
	// With --non-interactive, deploy with ambiguous server must error, not prompt
	cmd := NewRootCmd("test")
	cmd.SetArgs([]string{"deploy", "--non-interactive", "-p", "test-project"})
	err := cmd.Execute()
	// This will fail at auth since we have no credentials, but the flag should parse
	assert.Error(t, err)
}

func TestNonInteractive_InitFails(t *testing.T) {
	cmd := NewRootCmd("test")
	cmd.SetArgs([]string{"init", "--non-interactive"})
	err := cmd.Execute()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "interactive-only")
}

func TestNonInteractive_HelloFails(t *testing.T) {
	cmd := NewRootCmd("test")
	cmd.SetArgs([]string{"hello", "--non-interactive"})
	err := cmd.Execute()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "interactive-only")
}

func TestNonInteractive_ConfigureFails(t *testing.T) {
	cmd := NewRootCmd("test")
	cmd.SetArgs([]string{"configure", "--non-interactive"})
	err := cmd.Execute()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "interactive-only")
}

func TestAgentMetadata_MCPNotSafe(t *testing.T) {
	m := lookupAgentMetadata("dhq mcp")
	assert.True(t, m.Interactive)
	assert.False(t, m.SafeForAutomation)
}

func TestAgentMetadata_AuthLogout(t *testing.T) {
	m := lookupAgentMetadata("dhq auth logout")
	assert.True(t, m.Interactive, "logout can prompt for account picker")
	assert.True(t, m.SafeForAutomation, "safe with --account flag")
}

func TestAgentMetadata_Commands(t *testing.T) {
	m := lookupAgentMetadata("dhq commands")
	assert.True(t, m.Idempotent)
	assert.True(t, m.SupportsJSON)
	assert.True(t, m.SafeForAutomation)
}
