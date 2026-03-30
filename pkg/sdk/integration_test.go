package sdk

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRealProjectListShape validates our Project type against actual API response JSON.
// This JSON comes from a real call to the staging API.
func TestRealProjectListShape(t *testing.T) {
	realJSON := `[
		{
			"name": "Basic HTML",
			"permalink": "basic-html",
			"identifier": "93f658ea-24b1-47ae-b658-6ef927197d2d",
			"public_key": "ssh-ed25519 AAAAC3... DeployHQ.com Key for basic-html",
			"repository": {
				"scm_type": "git",
				"url": "https://github.com/thdurante/basic-html.git",
				"port": null,
				"username": null,
				"branch": "main",
				"cached": true,
				"hosting_service": {
					"name": "GitHub",
					"url": "http://github.com",
					"tree_url": "https://github.com/thdurante/basic-html/tree/main",
					"commits_url": "https://github.com/thdurante/basic-html/commits/main"
				}
			},
			"repository_url": "https://github.com/thdurante/basic-html.git",
			"zone": "uk",
			"last_deployed_at": "2026-02-26T13:43:29.000+01:00",
			"auto_deploy_url": "https://sg.staging.deployhq.com/deploy/basic-html/abc123",
			"starred": false
		},
		{
			"name": "Hostinger",
			"permalink": "hostinger",
			"identifier": "d451ef9a-b41f-4483-9a07-c589b3f6b8f3",
			"public_key": "ssh-ed25519 AAAAC3...",
			"repository": null,
			"repository_url": null,
			"zone": "uk",
			"last_deployed_at": null,
			"auto_deploy_url": "https://sg.staging.deployhq.com/deploy/hostinger/xyz789",
			"starred": false
		}
	]`

	var projects []Project
	err := json.Unmarshal([]byte(realJSON), &projects)
	require.NoError(t, err)
	require.Len(t, projects, 2)

	// Project with full data
	p := projects[0]
	assert.Equal(t, "Basic HTML", p.Name)
	assert.Equal(t, "basic-html", p.Permalink)
	assert.Equal(t, "93f658ea-24b1-47ae-b658-6ef927197d2d", p.Identifier)
	assert.Equal(t, "uk", p.Zone)
	assert.NotNil(t, p.Repository)
	assert.Equal(t, "git", p.Repository.ScmType)
	assert.Equal(t, "main", p.Repository.Branch)
	assert.True(t, p.Repository.Cached)
	assert.Nil(t, p.Repository.Port)
	assert.Nil(t, p.Repository.Username)
	assert.NotNil(t, p.Repository.HostingService)
	assert.Equal(t, "GitHub", p.Repository.HostingService.Name)
	assert.NotNil(t, p.LastDeployedAt)
	assert.NotNil(t, p.RepositoryURL)
	assert.False(t, p.Starred)

	// Project with nulls (no repo)
	p2 := projects[1]
	assert.Equal(t, "Hostinger", p2.Name)
	assert.Nil(t, p2.Repository)
	assert.Nil(t, p2.RepositoryURL)
	assert.Nil(t, p2.LastDeployedAt)
}

// TestRealDeploymentShape validates Deployment type against actual API shape.
func TestRealDeploymentShape(t *testing.T) {
	realJSON := `{
		"identifier": "abc-123-def",
		"servers": [
			{
				"id": 42,
				"identifier": "srv-001",
				"name": "Production",
				"protocol_type": "ssh",
				"server_path": "/var/www/html",
				"last_revision": "abc123",
				"preferred_branch": "main",
				"branch": "main",
				"notify_email": "",
				"server_group_identifier": null,
				"auto_deploy": false,
				"environment": "production",
				"enabled": true,
				"agent": null,
				"atomic": true,
				"atomic_strategy": "symlink",
				"atomic_retention": 5,
				"use_compression": true,
				"use_accelerated_transfer": false,
				"use_parallel_upload": false,
				"root_path": "/",
				"position": 1,
				"created_at": "2025-01-01T00:00:00.000+01:00",
				"updated_at": "2026-01-01T00:00:00.000+01:00",
				"connection_checked_at": null,
				"connection_error_message": null,
				"hostname": "example.com",
				"username": "deploy",
				"port": "22",
				"use_ssh_keys": true,
				"host_key": "",
				"unlink_before_upload": false
			}
		],
		"project": {
			"name": "My App",
			"permalink": "my-app",
			"identifier": "proj-123",
			"public_key": "ssh-ed25519 ...",
			"repository": {
				"scm_type": "git",
				"url": "git@github.com:user/repo.git",
				"port": null,
				"username": null,
				"branch": "main",
				"cached": true,
				"hosting_service": {
					"name": "GitHub",
					"url": "http://github.com",
					"tree_url": "https://github.com/user/repo/tree/main",
					"commits_url": "https://github.com/user/repo/commits/main"
				}
			},
			"repository_url": "git@github.com:user/repo.git",
			"zone": "uk",
			"last_deployed_at": "2026-01-01T00:00:00.000+01:00",
			"auto_deploy_url": "https://sg.staging.deployhq.com/deploy/my-app/token"
		},
		"deployer": "user@example.com",
		"deployer_avatar": "https://gravatar.com/avatar/abc",
		"branch": "main",
		"start_revision": { "ref": "aaa111" },
		"end_revision": { "ref": "bbb222" },
		"status": "completed",
		"timestamps": {
			"queued_at": "2026-01-01T12:00:00.000+01:00",
			"started_at": "2026-01-01T12:00:01.000+01:00",
			"completed_at": "2026-01-01T12:00:30.000+01:00",
			"duration": "29",
			"runs_at": "2026-01-01T12:00:00.000+01:00"
		},
		"configuration": {
			"copy_config_files": true,
			"notification_addresses": "admin@example.com",
			"skip_project_files": false
		},
		"legacy": false,
		"deferred": false,
		"config_files_deployment": false,
		"overview": null,
		"metadata": {},
		"archived": false,
		"archived_at": null,
		"log_summary": null,
		"steps": [
			{
				"step": "transfer",
				"stage": "deploy",
				"identifier": "step-001",
				"server": "srv-001",
				"total_items": "15",
				"completed_items": "15",
				"description": "Transferring files to Production",
				"status": "completed",
				"logs": true,
				"deployment_started_at": "2026-01-01T12:00:01.000+01:00",
				"updated_at": "2026-01-01T12:00:30.000+01:00"
			}
		]
	}`

	var dep Deployment
	err := json.Unmarshal([]byte(realJSON), &dep)
	require.NoError(t, err)

	assert.Equal(t, "abc-123-def", dep.Identifier)
	assert.Equal(t, "completed", dep.Status)
	assert.Equal(t, "main", dep.Branch)
	assert.NotNil(t, dep.Deployer)
	assert.Equal(t, "user@example.com", *dep.Deployer)

	// Servers
	require.Len(t, dep.Servers, 1)
	assert.Equal(t, "Production", dep.Servers[0].Name)
	assert.Equal(t, "ssh", dep.Servers[0].ProtocolType)
	assert.Equal(t, 42, dep.Servers[0].ID)
	assert.True(t, dep.Servers[0].Enabled)
	assert.Nil(t, dep.Servers[0].Agent)

	// Project
	assert.NotNil(t, dep.Project)
	assert.Equal(t, "My App", dep.Project.Name)

	// Revisions
	assert.NotNil(t, dep.StartRevision)
	assert.Equal(t, "aaa111", dep.StartRevision.Ref)
	assert.NotNil(t, dep.EndRevision)
	assert.Equal(t, "bbb222", dep.EndRevision.Ref)

	// Timestamps
	assert.NotNil(t, dep.Timestamps)
	assert.NotNil(t, dep.Timestamps.CompletedAt)
	assert.Equal(t, "29", dep.Timestamps.Duration.String())

	// Configuration
	assert.NotNil(t, dep.Configuration)
	assert.True(t, dep.Configuration.CopyConfigFiles)

	// Steps
	require.Len(t, dep.Steps, 1)
	assert.Equal(t, "transfer", dep.Steps[0].Step)
	assert.True(t, dep.Steps[0].Logs)
	assert.Equal(t, "15", dep.Steps[0].TotalItems.String())

	// Flags
	assert.False(t, dep.Legacy)
	assert.False(t, dep.Archived)
	assert.Nil(t, dep.Overview)
}

// TestRealServerShape validates Server type with all fields populated.
func TestRealServerShape(t *testing.T) {
	realJSON := `{
		"id": 42,
		"identifier": "srv-001",
		"name": "Production",
		"protocol_type": "ssh",
		"server_path": "/var/www/html",
		"last_revision": "abc123def456",
		"preferred_branch": "main",
		"branch": "main",
		"notify_email": "ops@example.com",
		"server_group_identifier": null,
		"auto_deploy": true,
		"environment": "production",
		"enabled": true,
		"agent": null,
		"atomic": true,
		"atomic_strategy": "symlink",
		"atomic_retention": 5,
		"use_compression": true,
		"use_accelerated_transfer": false,
		"use_parallel_upload": true,
		"root_path": "/",
		"position": 1,
		"created_at": "2025-01-01T00:00:00.000+01:00",
		"updated_at": "2026-03-01T00:00:00.000+01:00",
		"connection_checked_at": "2026-03-01T00:00:00.000+01:00",
		"connection_error_message": null
	}`

	var server Server
	err := json.Unmarshal([]byte(realJSON), &server)
	require.NoError(t, err)

	assert.Equal(t, 42, server.ID)
	assert.Equal(t, "Production", server.Name)
	assert.Equal(t, "ssh", server.ProtocolType)
	assert.True(t, server.AutoDeploy)
	assert.True(t, server.Enabled)
	assert.Nil(t, server.Agent) // null agent in JSON
	assert.Nil(t, server.ServerGroupIdentifier)
	assert.NotNil(t, server.ConnectionCheckedAt)
	assert.Nil(t, server.ConnectionErrorMessage)
	assert.True(t, server.UseCompression)
	assert.True(t, server.UseParallelUpload)
}

// TestRealServerWithAgent validates Server deserializes when agent is an object.
func TestRealServerWithAgent(t *testing.T) {
	realJSON := `{
		"id": 99,
		"identifier": "srv-agent",
		"name": "dhq-agent",
		"protocol_type": "ssh",
		"server_path": "/var/www",
		"last_revision": "",
		"preferred_branch": "main",
		"branch": "main",
		"notify_email": "",
		"server_group_identifier": null,
		"auto_deploy": false,
		"environment": "",
		"enabled": true,
		"agent": {
			"created_at": "2025-11-25T11:41:17.000Z",
			"id": 6000,
			"identifier": "54fc66d1-c077-4f0b-9617-b1ad8552b5e1",
			"name": "2511",
			"online": false,
			"revoked_at": null,
			"updated_at": "2025-11-25T11:41:36.000Z"
		},
		"atomic": null,
		"atomic_strategy": "",
		"atomic_retention": 0,
		"use_compression": false,
		"use_accelerated_transfer": false,
		"use_parallel_upload": false,
		"root_path": "",
		"position": 1,
		"created_at": "2025-11-25T11:42:00.000Z",
		"updated_at": "2025-11-25T11:42:00.000Z",
		"connection_checked_at": null,
		"connection_error_message": null
	}`

	var server Server
	err := json.Unmarshal([]byte(realJSON), &server)
	require.NoError(t, err)

	assert.Equal(t, "dhq-agent", server.Name)
	assert.NotNil(t, server.Agent)
	assert.Equal(t, "2511", server.Agent.Name)
	assert.Equal(t, 6000, server.Agent.ID)
	assert.False(t, server.Agent.Online)
	assert.Nil(t, server.Agent.RevokedAt)
}
