package sdk

import (
	"context"
	"fmt"
)

// Template sub-resources mirror the project-level resources, but are nested
// under /templates/:template_id/... where :template_id is the template's
// string permalink. Sub-resource items are keyed by their string identifier
// (or, for build_languages, by the package name).
//
// Types are reused from the project-level equivalents wherever the shape is
// identical (ConfigFile, ExcludedFile, SSHCommand, BuildCommand,
// BuildCacheFile, BuildKnownHost, Integration, Server, BuildConfig,
// BuildLanguage). New request types are defined only where the template
// variant genuinely differs (server groups gain extra fields).

// --- 1. config_files (full CRUD) ---

func (c *Client) ListTemplateConfigFiles(ctx context.Context, templatePermalink string, opts *ListOptions) ([]ConfigFile, error) {
	var files []ConfigFile
	path := appendListParams(fmt.Sprintf("/templates/%s/config_files", templatePermalink), opts)
	if err := c.get(ctx, path, &files); err != nil {
		return nil, err
	}
	return files, nil
}

func (c *Client) GetTemplateConfigFile(ctx context.Context, templatePermalink, fileID string) (*ConfigFile, error) {
	var file ConfigFile
	if err := c.get(ctx, fmt.Sprintf("/templates/%s/config_files/%s", templatePermalink, fileID), &file); err != nil {
		return nil, err
	}
	return &file, nil
}

func (c *Client) CreateTemplateConfigFile(ctx context.Context, templatePermalink string, req ConfigFileCreateRequest) (*ConfigFile, error) {
	body := struct {
		ConfigFile ConfigFileCreateRequest `json:"config_file"`
	}{ConfigFile: req}
	var file ConfigFile
	if err := c.post(ctx, fmt.Sprintf("/templates/%s/config_files", templatePermalink), body, &file); err != nil {
		return nil, err
	}
	return &file, nil
}

func (c *Client) UpdateTemplateConfigFile(ctx context.Context, templatePermalink, fileID string, req ConfigFileUpdateRequest) (*ConfigFile, error) {
	body := struct {
		ConfigFile ConfigFileUpdateRequest `json:"config_file"`
	}{ConfigFile: req}
	var file ConfigFile
	if err := c.put(ctx, fmt.Sprintf("/templates/%s/config_files/%s", templatePermalink, fileID), body, &file); err != nil {
		return nil, err
	}
	return &file, nil
}

func (c *Client) DeleteTemplateConfigFile(ctx context.Context, templatePermalink, fileID string) error {
	return c.delete(ctx, fmt.Sprintf("/templates/%s/config_files/%s", templatePermalink, fileID))
}

// --- 2. excluded_files (index/create/update/destroy — NO show) ---

func (c *Client) ListTemplateExcludedFiles(ctx context.Context, templatePermalink string, opts *ListOptions) ([]ExcludedFile, error) {
	var files []ExcludedFile
	path := appendListParams(fmt.Sprintf("/templates/%s/excluded_files", templatePermalink), opts)
	if err := c.get(ctx, path, &files); err != nil {
		return nil, err
	}
	return files, nil
}

func (c *Client) CreateTemplateExcludedFile(ctx context.Context, templatePermalink string, req ExcludedFileCreateRequest) (*ExcludedFile, error) {
	body := struct {
		ExcludedFile ExcludedFileCreateRequest `json:"excluded_file"`
	}{ExcludedFile: req}
	var file ExcludedFile
	if err := c.post(ctx, fmt.Sprintf("/templates/%s/excluded_files", templatePermalink), body, &file); err != nil {
		return nil, err
	}
	return &file, nil
}

func (c *Client) UpdateTemplateExcludedFile(ctx context.Context, templatePermalink, fileID string, req ExcludedFileCreateRequest) (*ExcludedFile, error) {
	body := struct {
		ExcludedFile ExcludedFileCreateRequest `json:"excluded_file"`
	}{ExcludedFile: req}
	var file ExcludedFile
	if err := c.put(ctx, fmt.Sprintf("/templates/%s/excluded_files/%s", templatePermalink, fileID), body, &file); err != nil {
		return nil, err
	}
	return &file, nil
}

func (c *Client) DeleteTemplateExcludedFile(ctx context.Context, templatePermalink, fileID string) error {
	return c.delete(ctx, fmt.Sprintf("/templates/%s/excluded_files/%s", templatePermalink, fileID))
}

// --- 3. integrations (index/create/update/destroy) ---
// Delta from project-level: create is limited to ALLOWED_API_TYPES server-side
// (422 otherwise); hook_type is immutable on update; external-auth hook types
// may return {auth_required, auth_url} instead of an integration — surfaced
// via the raw create/update response when present.

func (c *Client) ListTemplateIntegrations(ctx context.Context, templatePermalink string, opts *ListOptions) ([]Integration, error) {
	var integrations []Integration
	path := appendListParams(fmt.Sprintf("/templates/%s/integrations", templatePermalink), opts)
	if err := c.get(ctx, path, &integrations); err != nil {
		return nil, err
	}
	return integrations, nil
}

func (c *Client) CreateTemplateIntegration(ctx context.Context, templatePermalink string, req IntegrationCreateRequest) (*Integration, error) {
	body := struct {
		Integration IntegrationCreateRequest `json:"integration"`
	}{Integration: req}
	var integration Integration
	if err := c.post(ctx, fmt.Sprintf("/templates/%s/integrations", templatePermalink), body, &integration); err != nil {
		return nil, err
	}
	return &integration, nil
}

func (c *Client) UpdateTemplateIntegration(ctx context.Context, templatePermalink, integrationID string, req IntegrationCreateRequest) (*Integration, error) {
	body := struct {
		Integration IntegrationCreateRequest `json:"integration"`
	}{Integration: req}
	var integration Integration
	if err := c.put(ctx, fmt.Sprintf("/templates/%s/integrations/%s", templatePermalink, integrationID), body, &integration); err != nil {
		return nil, err
	}
	return &integration, nil
}

func (c *Client) DeleteTemplateIntegration(ctx context.Context, templatePermalink, integrationID string) error {
	return c.delete(ctx, fmt.Sprintf("/templates/%s/integrations/%s", templatePermalink, integrationID))
}

// --- 4. commands (index/create/update/destroy) ---
// Wrapper key "command"; reuses SSHCommand/SSHCommandCreateRequest.
// NOTE: /templates/:id/ssh_commands is a DUPLICATE route to the same
// controller; the CLI exposes `dhq templates commands` (with an
// `ssh-commands` alias). These methods drive both routes identically.

func (c *Client) ListTemplateCommands(ctx context.Context, templatePermalink string, opts *ListOptions) ([]SSHCommand, error) {
	var cmds []SSHCommand
	path := appendListParams(fmt.Sprintf("/templates/%s/commands", templatePermalink), opts)
	if err := c.get(ctx, path, &cmds); err != nil {
		return nil, err
	}
	return cmds, nil
}

func (c *Client) CreateTemplateCommand(ctx context.Context, templatePermalink string, req SSHCommandCreateRequest) (*SSHCommand, error) {
	body := struct {
		Command SSHCommandCreateRequest `json:"command"`
	}{Command: req}
	var cmd SSHCommand
	if err := c.post(ctx, fmt.Sprintf("/templates/%s/commands", templatePermalink), body, &cmd); err != nil {
		return nil, err
	}
	return &cmd, nil
}

func (c *Client) UpdateTemplateCommand(ctx context.Context, templatePermalink, cmdID string, req SSHCommandCreateRequest) (*SSHCommand, error) {
	body := struct {
		Command SSHCommandCreateRequest `json:"command"`
	}{Command: req}
	var cmd SSHCommand
	if err := c.put(ctx, fmt.Sprintf("/templates/%s/commands/%s", templatePermalink, cmdID), body, &cmd); err != nil {
		return nil, err
	}
	return &cmd, nil
}

func (c *Client) DeleteTemplateCommand(ctx context.Context, templatePermalink, cmdID string) error {
	return c.delete(ctx, fmt.Sprintf("/templates/%s/commands/%s", templatePermalink, cmdID))
}

// --- 5. build_commands (index/create/update/destroy) ---
// Delta: server-side failures surface as 500 (not 422). Just surface the error.

func (c *Client) ListTemplateBuildCommands(ctx context.Context, templatePermalink string, opts *ListOptions) ([]BuildCommand, error) {
	var cmds []BuildCommand
	path := appendListParams(fmt.Sprintf("/templates/%s/build_commands", templatePermalink), opts)
	if err := c.get(ctx, path, &cmds); err != nil {
		return nil, err
	}
	return cmds, nil
}

func (c *Client) CreateTemplateBuildCommand(ctx context.Context, templatePermalink string, req BuildCommandCreateRequest) (*BuildCommand, error) {
	body := struct {
		BuildCommand BuildCommandCreateRequest `json:"build_command"`
	}{BuildCommand: req}
	var cmd BuildCommand
	if err := c.post(ctx, fmt.Sprintf("/templates/%s/build_commands", templatePermalink), body, &cmd); err != nil {
		return nil, err
	}
	return &cmd, nil
}

func (c *Client) UpdateTemplateBuildCommand(ctx context.Context, templatePermalink, cmdID string, req BuildCommandCreateRequest) (*BuildCommand, error) {
	body := struct {
		BuildCommand BuildCommandCreateRequest `json:"build_command"`
	}{BuildCommand: req}
	var cmd BuildCommand
	if err := c.put(ctx, fmt.Sprintf("/templates/%s/build_commands/%s", templatePermalink, cmdID), body, &cmd); err != nil {
		return nil, err
	}
	return &cmd, nil
}

func (c *Client) DeleteTemplateBuildCommand(ctx context.Context, templatePermalink, cmdID string) error {
	return c.delete(ctx, fmt.Sprintf("/templates/%s/build_commands/%s", templatePermalink, cmdID))
}

// --- 6. build_cache_files (index/create/update/destroy) ---
// Delta: 500 on server-side failure.

func (c *Client) ListTemplateBuildCacheFiles(ctx context.Context, templatePermalink string) ([]BuildCacheFile, error) {
	var files []BuildCacheFile
	if err := c.get(ctx, fmt.Sprintf("/templates/%s/build_cache_files", templatePermalink), &files); err != nil {
		return nil, err
	}
	return files, nil
}

func (c *Client) CreateTemplateBuildCacheFile(ctx context.Context, templatePermalink string, req BuildCacheFileCreateRequest) (*BuildCacheFile, error) {
	body := struct {
		BuildCacheFile BuildCacheFileCreateRequest `json:"build_cache_file"`
	}{BuildCacheFile: req}
	var file BuildCacheFile
	if err := c.post(ctx, fmt.Sprintf("/templates/%s/build_cache_files", templatePermalink), body, &file); err != nil {
		return nil, err
	}
	return &file, nil
}

func (c *Client) UpdateTemplateBuildCacheFile(ctx context.Context, templatePermalink, fileID string, req BuildCacheFileCreateRequest) (*BuildCacheFile, error) {
	body := struct {
		BuildCacheFile BuildCacheFileCreateRequest `json:"build_cache_file"`
	}{BuildCacheFile: req}
	var file BuildCacheFile
	if err := c.put(ctx, fmt.Sprintf("/templates/%s/build_cache_files/%s", templatePermalink, fileID), body, &file); err != nil {
		return nil, err
	}
	return &file, nil
}

func (c *Client) DeleteTemplateBuildCacheFile(ctx context.Context, templatePermalink, fileID string) error {
	return c.delete(ctx, fmt.Sprintf("/templates/%s/build_cache_files/%s", templatePermalink, fileID))
}

// --- 7. build_known_hosts (index/create/destroy — NO update) ---
// Delta: 500 on create failure.

func (c *Client) ListTemplateBuildKnownHosts(ctx context.Context, templatePermalink string) ([]BuildKnownHost, error) {
	var hosts []BuildKnownHost
	if err := c.get(ctx, fmt.Sprintf("/templates/%s/build_known_hosts", templatePermalink), &hosts); err != nil {
		return nil, err
	}
	return hosts, nil
}

func (c *Client) CreateTemplateBuildKnownHost(ctx context.Context, templatePermalink string, req BuildKnownHostCreateRequest) (*BuildKnownHost, error) {
	body := struct {
		BuildKnownHost BuildKnownHostCreateRequest `json:"build_known_host"`
	}{BuildKnownHost: req}
	var host BuildKnownHost
	if err := c.post(ctx, fmt.Sprintf("/templates/%s/build_known_hosts", templatePermalink), body, &host); err != nil {
		return nil, err
	}
	return &host, nil
}

func (c *Client) DeleteTemplateBuildKnownHost(ctx context.Context, templatePermalink, hostID string) error {
	return c.delete(ctx, fmt.Sprintf("/templates/%s/build_known_hosts/%s", templatePermalink, hostID))
}

// --- 8. build_languages (UPDATE ONLY) ---
// PATCH /templates/:id/build_languages/:package_name — targets the template's
// default build environment. Wrapper key "build_environment" with just
// `version`. The :id path segment is the package NAME (e.g. "ruby", "nodejs").

func (c *Client) UpdateTemplateBuildLanguage(ctx context.Context, templatePermalink, packageName string, req BuildLanguageUpdateRequest) (*BuildLanguage, error) {
	body := struct {
		BuildEnvironment BuildLanguageUpdateRequest `json:"build_environment"`
	}{BuildEnvironment: req}
	var lang BuildLanguage
	path := fmt.Sprintf("/templates/%s/build_languages/%s", templatePermalink, packageName)
	if err := c.patch(ctx, path, body, &lang); err != nil {
		return nil, err
	}
	return &lang, nil
}

// --- 9. build_configuration (SHOW ONLY, singular route) ---
// GET /templates/:id/build_configuration (no :id) — returns the template's
// default build environment api hash.

func (c *Client) GetTemplateBuildConfiguration(ctx context.Context, templatePermalink string) (*BuildConfig, error) {
	var config BuildConfig
	if err := c.get(ctx, fmt.Sprintf("/templates/%s/build_configuration", templatePermalink), &config); err != nil {
		return nil, err
	}
	return &config, nil
}

// --- 10. servers (full CRUD) ---
// Reuses the project-level Server type and request shapes.
// Delta: PROJECT_ONLY_PROTOCOLS are rejected 422 server-side; the API also
// accepts a `protocol_type` alias (already the field name we send).

func (c *Client) ListTemplateServers(ctx context.Context, templatePermalink string, opts *ListOptions) ([]Server, error) {
	var servers []Server
	path := appendListParams(fmt.Sprintf("/templates/%s/servers", templatePermalink), opts)
	if err := c.get(ctx, path, &servers); err != nil {
		return nil, err
	}
	return servers, nil
}

func (c *Client) GetTemplateServer(ctx context.Context, templatePermalink, serverID string) (*Server, error) {
	var server Server
	if err := c.get(ctx, fmt.Sprintf("/templates/%s/servers/%s", templatePermalink, serverID), &server); err != nil {
		return nil, err
	}
	return &server, nil
}

func (c *Client) CreateTemplateServer(ctx context.Context, templatePermalink string, req ServerCreateRequest) (*Server, error) {
	body := struct {
		Server ServerCreateRequest `json:"server"`
	}{Server: req}
	var server Server
	if err := c.post(ctx, fmt.Sprintf("/templates/%s/servers", templatePermalink), body, &server); err != nil {
		return nil, err
	}
	return &server, nil
}

func (c *Client) UpdateTemplateServer(ctx context.Context, templatePermalink, serverID string, req ServerUpdateRequest) (*Server, error) {
	body := struct {
		Server ServerUpdateRequest `json:"server"`
	}{Server: req}
	var server Server
	if err := c.put(ctx, fmt.Sprintf("/templates/%s/servers/%s", templatePermalink, serverID), body, &server); err != nil {
		return nil, err
	}
	return &server, nil
}

func (c *Client) DeleteTemplateServer(ctx context.Context, templatePermalink, serverID string) error {
	return c.delete(ctx, fmt.Sprintf("/templates/%s/servers/%s", templatePermalink, serverID))
}

// --- 11. server_groups (index/create/update/destroy — NO show) ---
// The template variant accepts more fields than the project-level create
// (name, auto_deploy, transfer_order, email_notify_on, notification_email,
// environment), so it uses dedicated request types.

// TemplateServerGroupCreateRequest is the payload for creating a template
// server group. Wrapper key "server_group".
type TemplateServerGroupCreateRequest struct {
	Name              string `json:"name"`
	AutoDeploy        *bool  `json:"auto_deploy,omitempty"`
	TransferOrder     string `json:"transfer_order,omitempty"`
	EmailNotifyOn     string `json:"email_notify_on,omitempty"`
	NotificationEmail string `json:"notification_email,omitempty"`
	Environment       string `json:"environment,omitempty"`
}

// TemplateServerGroupUpdateRequest is the payload for updating a template
// server group. All fields are optional.
type TemplateServerGroupUpdateRequest struct {
	Name              string `json:"name,omitempty"`
	AutoDeploy        *bool  `json:"auto_deploy,omitempty"`
	TransferOrder     string `json:"transfer_order,omitempty"`
	EmailNotifyOn     string `json:"email_notify_on,omitempty"`
	NotificationEmail string `json:"notification_email,omitempty"`
	Environment       string `json:"environment,omitempty"`
}

func (c *Client) ListTemplateServerGroups(ctx context.Context, templatePermalink string, opts *ListOptions) ([]ServerGroup, error) {
	var groups []ServerGroup
	path := appendListParams(fmt.Sprintf("/templates/%s/server_groups", templatePermalink), opts)
	if err := c.get(ctx, path, &groups); err != nil {
		return nil, err
	}
	return groups, nil
}

func (c *Client) CreateTemplateServerGroup(ctx context.Context, templatePermalink string, req TemplateServerGroupCreateRequest) (*ServerGroup, error) {
	body := struct {
		ServerGroup TemplateServerGroupCreateRequest `json:"server_group"`
	}{ServerGroup: req}
	var group ServerGroup
	if err := c.post(ctx, fmt.Sprintf("/templates/%s/server_groups", templatePermalink), body, &group); err != nil {
		return nil, err
	}
	return &group, nil
}

func (c *Client) UpdateTemplateServerGroup(ctx context.Context, templatePermalink, groupID string, req TemplateServerGroupUpdateRequest) (*ServerGroup, error) {
	body := struct {
		ServerGroup TemplateServerGroupUpdateRequest `json:"server_group"`
	}{ServerGroup: req}
	var group ServerGroup
	if err := c.put(ctx, fmt.Sprintf("/templates/%s/server_groups/%s", templatePermalink, groupID), body, &group); err != nil {
		return nil, err
	}
	return &group, nil
}

func (c *Client) DeleteTemplateServerGroup(ctx context.Context, templatePermalink, groupID string) error {
	return c.delete(ctx, fmt.Sprintf("/templates/%s/server_groups/%s", templatePermalink, groupID))
}
