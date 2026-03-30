package commands

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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
