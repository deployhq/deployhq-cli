package commands

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCommandsCatalog_RequiredFlags(t *testing.T) {
	root := NewRootCmd("test")
	catalog := buildCatalog(root)

	// Find "servers" → "create" which has --name (required) and --protocol-type (required)
	var serversCreate *CommandInfo
	for i := range catalog {
		if catalog[i].Name == "servers" {
			for j := range catalog[i].Subcommands {
				if catalog[i].Subcommands[j].Name == "create" {
					serversCreate = &catalog[i].Subcommands[j]
					break
				}
			}
		}
	}

	require.NotNil(t, serversCreate, "servers create command should exist in catalog")

	requiredFlags := map[string]bool{}
	for _, f := range serversCreate.Flags {
		if f.Required {
			requiredFlags[f.Name] = true
		}
	}
	assert.True(t, requiredFlags["name"], "--name should be marked required")
	assert.True(t, requiredFlags["protocol-type"], "--protocol-type should be marked required")
}

func TestCommandsCatalog_InheritedFlags(t *testing.T) {
	root := NewRootCmd("test")
	catalog := buildCatalog(root)

	// Find "servers" → "list" which should inherit --project and --json
	var serversList *CommandInfo
	for i := range catalog {
		if catalog[i].Name == "servers" {
			for j := range catalog[i].Subcommands {
				if catalog[i].Subcommands[j].Name == "list" {
					serversList = &catalog[i].Subcommands[j]
					break
				}
			}
		}
	}

	require.NotNil(t, serversList, "servers list command should exist in catalog")

	flagNames := map[string]bool{}
	for _, f := range serversList.Flags {
		flagNames[f.Name] = true
	}
	assert.True(t, flagNames["project"], "--project should appear as inherited flag")
	assert.True(t, flagNames["json"], "--json should appear as inherited flag")
}

func TestCommandsCatalog_Registered(t *testing.T) {
	cmd := NewRootCmd("test")
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetArgs([]string{"--help"})

	err := cmd.Execute()
	require.NoError(t, err)

	out := stdout.String()
	assert.Contains(t, out, "commands")
	assert.Contains(t, out, "show")
	assert.Contains(t, out, "url")
	assert.Contains(t, out, "setup")
}

func TestSetupCommand_Help(t *testing.T) {
	cmd := NewRootCmd("test")
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetArgs([]string{"setup", "--help"})

	err := cmd.Execute()
	require.NoError(t, err)

	out := stdout.String()
	assert.Contains(t, out, "claude")
	assert.Contains(t, out, "codex")
}

func TestURLParseCommand_Help(t *testing.T) {
	cmd := NewRootCmd("test")
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetArgs([]string{"url", "--help"})

	err := cmd.Execute()
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "parse")
}

func TestParseDeployHQURL(t *testing.T) {
	tests := []struct {
		name        string
		url         string
		account     string
		resource    string
		project     string
		subResource string
		id          string
	}{
		{
			name: "projects list",
			url:  "https://myco.deployhq.com/projects",
			account: "myco", resource: "projects",
		},
		{
			name: "project show",
			url:  "https://myco.deployhq.com/projects/my-app",
			account: "myco", resource: "projects", project: "my-app",
		},
		{
			name: "deployment show",
			url:  "https://myco.deployhq.com/projects/my-app/deployments/abc123",
			account: "myco", resource: "projects", project: "my-app",
			subResource: "deployments", id: "abc123",
		},
		{
			name: "servers",
			url:  "https://myco.deployhq.com/projects/my-app/servers",
			account: "myco", resource: "projects", project: "my-app",
			subResource: "servers",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parsed, err := parseDeployHQURL(tt.url)
			require.NoError(t, err)
			assert.Equal(t, tt.account, parsed.Account)
			assert.Equal(t, tt.resource, parsed.Resource)
			assert.Equal(t, tt.project, parsed.Project)
			assert.Equal(t, tt.subResource, parsed.SubResource)
			assert.Equal(t, tt.id, parsed.ID)
		})
	}
}

func TestParseDeployHQURL_InvalidHost(t *testing.T) {
	_, err := parseDeployHQURL("https://example.com/projects")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Not a DeployHQ URL")
}

func TestBuildAPIPath(t *testing.T) {
	tests := []struct {
		parsed   ParsedURL
		expected string
	}{
		{ParsedURL{Resource: "projects"}, "/projects"},
		{ParsedURL{Resource: "projects", Project: "my-app"}, "/projects/my-app"},
		{ParsedURL{Resource: "projects", Project: "my-app", SubResource: "servers"}, "/projects/my-app/servers"},
		{ParsedURL{Resource: "projects", Project: "my-app", SubResource: "deployments", ID: "abc"}, "/projects/my-app/deployments/abc"},
	}

	for _, tt := range tests {
		assert.Equal(t, tt.expected, buildAPIPath(&tt.parsed))
	}
}
