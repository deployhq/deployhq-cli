package sdk

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// These tests focus on path correctness, the request wrapper key, and the
// per-sub-resource deltas (update-only build_languages with a package-name id,
// show-only build_configuration, no-update build_known_hosts). Field-level
// coverage is delegated to the project-level tests since the types are reused.

// --- 1. config_files ---

func TestListTemplateConfigFiles(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/templates/my-tmpl/config_files", r.URL.Path)
		_ = json.NewEncoder(w).Encode([]ConfigFile{{Identifier: "cf1", Path: "/etc/app.conf"}})
	}))
	defer srv.Close()

	c := newTestClient(t, srv)
	files, err := c.ListTemplateConfigFiles(context.Background(), "my-tmpl", nil)
	require.NoError(t, err)
	assert.Len(t, files, 1)
	assert.Equal(t, "/etc/app.conf", files[0].Path)
}

func TestCreateTemplateConfigFile(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/templates/my-tmpl/config_files", r.URL.Path)
		var body struct {
			ConfigFile ConfigFileCreateRequest `json:"config_file"`
		}
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		assert.Equal(t, "/etc/app.conf", body.ConfigFile.Path)
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(ConfigFile{Identifier: "cf1", Path: body.ConfigFile.Path})
	}))
	defer srv.Close()

	c := newTestClient(t, srv)
	f, err := c.CreateTemplateConfigFile(context.Background(), "my-tmpl", ConfigFileCreateRequest{Path: "/etc/app.conf", Body: "x"})
	require.NoError(t, err)
	assert.Equal(t, "/etc/app.conf", f.Path)
}

func TestUpdateTemplateConfigFile_DescriptionOnly(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPut, r.Method)
		assert.Equal(t, "/templates/my-tmpl/config_files/cf1", r.URL.Path)
		// A description-only update must NOT send empty path/body, which would
		// clear the existing values server-side.
		var raw map[string]map[string]interface{}
		require.NoError(t, json.NewDecoder(r.Body).Decode(&raw))
		cf := raw["config_file"]
		assert.Equal(t, "new desc", cf["description"])
		_, hasPath := cf["path"]
		_, hasBody := cf["body"]
		assert.False(t, hasPath, "path must be omitted when unchanged")
		assert.False(t, hasBody, "body must be omitted when unchanged")
		_ = json.NewEncoder(w).Encode(ConfigFile{Identifier: "cf1", Path: "/etc/app.conf", Description: "new desc"})
	}))
	defer srv.Close()

	desc := "new desc"
	c := newTestClient(t, srv)
	f, err := c.UpdateTemplateConfigFile(context.Background(), "my-tmpl", "cf1", ConfigFileUpdateRequest{Description: &desc})
	require.NoError(t, err)
	assert.Equal(t, "new desc", f.Description)
}

func TestGetTemplateConfigFile(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/templates/my-tmpl/config_files/cf1", r.URL.Path)
		_ = json.NewEncoder(w).Encode(ConfigFile{Identifier: "cf1", Path: "/etc/app.conf"})
	}))
	defer srv.Close()

	c := newTestClient(t, srv)
	f, err := c.GetTemplateConfigFile(context.Background(), "my-tmpl", "cf1")
	require.NoError(t, err)
	assert.Equal(t, "cf1", f.Identifier)
}

func TestDeleteTemplateConfigFile(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodDelete, r.Method)
		assert.Equal(t, "/templates/my-tmpl/config_files/cf1", r.URL.Path)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	c := newTestClient(t, srv)
	require.NoError(t, c.DeleteTemplateConfigFile(context.Background(), "my-tmpl", "cf1"))
}

// --- 2. excluded_files ---

func TestListTemplateExcludedFiles(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/templates/my-tmpl/excluded_files", r.URL.Path)
		_ = json.NewEncoder(w).Encode([]ExcludedFile{{Identifier: "ef1", Path: "*.log"}})
	}))
	defer srv.Close()

	c := newTestClient(t, srv)
	files, err := c.ListTemplateExcludedFiles(context.Background(), "my-tmpl", nil)
	require.NoError(t, err)
	assert.Equal(t, "*.log", files[0].Path)
}

func TestCreateTemplateExcludedFile(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/templates/my-tmpl/excluded_files", r.URL.Path)
		var body struct {
			ExcludedFile ExcludedFileCreateRequest `json:"excluded_file"`
		}
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		assert.Equal(t, "*.log", body.ExcludedFile.Path)
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(ExcludedFile{Identifier: "ef1", Path: body.ExcludedFile.Path})
	}))
	defer srv.Close()

	c := newTestClient(t, srv)
	f, err := c.CreateTemplateExcludedFile(context.Background(), "my-tmpl", ExcludedFileCreateRequest{Path: "*.log"})
	require.NoError(t, err)
	assert.Equal(t, "*.log", f.Path)
}

func TestDeleteTemplateExcludedFile(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodDelete, r.Method)
		assert.Equal(t, "/templates/my-tmpl/excluded_files/ef1", r.URL.Path)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	c := newTestClient(t, srv)
	require.NoError(t, c.DeleteTemplateExcludedFile(context.Background(), "my-tmpl", "ef1"))
}

// --- 3. integrations ---

func TestListTemplateIntegrations(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/templates/my-tmpl/integrations", r.URL.Path)
		_ = json.NewEncoder(w).Encode([]Integration{{Identifier: "in1", HookType: "slack"}})
	}))
	defer srv.Close()

	c := newTestClient(t, srv)
	ints, err := c.ListTemplateIntegrations(context.Background(), "my-tmpl", nil)
	require.NoError(t, err)
	assert.Equal(t, "slack", ints[0].HookType)
}

func TestCreateTemplateIntegration(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/templates/my-tmpl/integrations", r.URL.Path)
		var body struct {
			Integration IntegrationCreateRequest `json:"integration"`
		}
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		assert.Equal(t, "slack", body.Integration.HookType)
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(Integration{Identifier: "in1", HookType: body.Integration.HookType})
	}))
	defer srv.Close()

	c := newTestClient(t, srv)
	in, err := c.CreateTemplateIntegration(context.Background(), "my-tmpl", IntegrationCreateRequest{HookType: "slack"})
	require.NoError(t, err)
	assert.Equal(t, "slack", in.HookType)
}

func TestCreateTemplateIntegration_ExternalAuth(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/templates/my-tmpl/integrations", r.URL.Path)
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"auth_required":true,"auth_url":"https://example.com/oauth"}`))
	}))
	defer srv.Close()

	c := newTestClient(t, srv)
	in, err := c.CreateTemplateIntegration(context.Background(), "my-tmpl", IntegrationCreateRequest{HookType: "github"})
	require.NoError(t, err)
	require.NotNil(t, in.AuthRequired)
	assert.True(t, *in.AuthRequired)
	assert.Equal(t, "https://example.com/oauth", in.AuthURL)
}

func TestUpdateTemplateIntegration_OmitsHookType(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPut, r.Method)
		assert.Equal(t, "/templates/my-tmpl/integrations/in1", r.URL.Path)
		// hook_type is immutable on update and must not be sent (even empty).
		var raw map[string]map[string]interface{}
		require.NoError(t, json.NewDecoder(r.Body).Decode(&raw))
		_, hasHookType := raw["integration"]["hook_type"]
		assert.False(t, hasHookType, "hook_type must be omitted on update")
		assert.Equal(t, "New Name", raw["integration"]["name"])
		_ = json.NewEncoder(w).Encode(Integration{Identifier: "in1", Name: "New Name"})
	}))
	defer srv.Close()

	c := newTestClient(t, srv)
	in, err := c.UpdateTemplateIntegration(context.Background(), "my-tmpl", "in1", IntegrationCreateRequest{Name: "New Name"})
	require.NoError(t, err)
	assert.Equal(t, "in1", in.Identifier)
}

// --- 4. commands ---

func TestListTemplateCommands(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/templates/my-tmpl/commands", r.URL.Path)
		_ = json.NewEncoder(w).Encode([]SSHCommand{{Identifier: "cmd1", Command: "ls"}})
	}))
	defer srv.Close()

	c := newTestClient(t, srv)
	cmds, err := c.ListTemplateCommands(context.Background(), "my-tmpl", nil)
	require.NoError(t, err)
	assert.Equal(t, "ls", cmds[0].Command)
}

func TestCreateTemplateCommand(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/templates/my-tmpl/commands", r.URL.Path)
		var body struct {
			Command SSHCommandCreateRequest `json:"command"`
		}
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		assert.Equal(t, "ls -la", body.Command.Command)
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(SSHCommand{Identifier: "cmd1", Command: body.Command.Command})
	}))
	defer srv.Close()

	c := newTestClient(t, srv)
	cmd, err := c.CreateTemplateCommand(context.Background(), "my-tmpl", SSHCommandCreateRequest{Command: "ls -la"})
	require.NoError(t, err)
	assert.Equal(t, "ls -la", cmd.Command)
}

// --- 5. build_commands ---

func TestListTemplateBuildCommands(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/templates/my-tmpl/build_commands", r.URL.Path)
		_ = json.NewEncoder(w).Encode([]BuildCommand{{Identifier: "bc1", Command: "npm ci"}})
	}))
	defer srv.Close()

	c := newTestClient(t, srv)
	cmds, err := c.ListTemplateBuildCommands(context.Background(), "my-tmpl", nil)
	require.NoError(t, err)
	assert.Equal(t, "npm ci", cmds[0].Command)
}

func TestCreateTemplateBuildCommand(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/templates/my-tmpl/build_commands", r.URL.Path)
		var body struct {
			BuildCommand BuildCommandCreateRequest `json:"build_command"`
		}
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		assert.Equal(t, "npm run build", body.BuildCommand.Command)
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(BuildCommand{Identifier: "bc1", Command: body.BuildCommand.Command})
	}))
	defer srv.Close()

	c := newTestClient(t, srv)
	cmd, err := c.CreateTemplateBuildCommand(context.Background(), "my-tmpl", BuildCommandCreateRequest{Command: "npm run build"})
	require.NoError(t, err)
	assert.Equal(t, "npm run build", cmd.Command)
}

// --- 6. build_cache_files ---

func TestListTemplateBuildCacheFiles(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/templates/my-tmpl/build_cache_files", r.URL.Path)
		_ = json.NewEncoder(w).Encode([]BuildCacheFile{{Identifier: "bcf1", Path: "node_modules"}})
	}))
	defer srv.Close()

	c := newTestClient(t, srv)
	files, err := c.ListTemplateBuildCacheFiles(context.Background(), "my-tmpl")
	require.NoError(t, err)
	assert.Equal(t, "node_modules", files[0].Path)
}

func TestCreateTemplateBuildCacheFile(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/templates/my-tmpl/build_cache_files", r.URL.Path)
		var body struct {
			BuildCacheFile BuildCacheFileCreateRequest `json:"build_cache_file"`
		}
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		assert.Equal(t, "vendor", body.BuildCacheFile.Path)
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(BuildCacheFile{Identifier: "bcf1", Path: body.BuildCacheFile.Path})
	}))
	defer srv.Close()

	c := newTestClient(t, srv)
	f, err := c.CreateTemplateBuildCacheFile(context.Background(), "my-tmpl", BuildCacheFileCreateRequest{Path: "vendor"})
	require.NoError(t, err)
	assert.Equal(t, "vendor", f.Path)
}

// --- 7. build_known_hosts (no update) ---

func TestListTemplateBuildKnownHosts(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/templates/my-tmpl/build_known_hosts", r.URL.Path)
		_ = json.NewEncoder(w).Encode([]BuildKnownHost{{Identifier: "kh1", Hostname: "github.com"}})
	}))
	defer srv.Close()

	c := newTestClient(t, srv)
	hosts, err := c.ListTemplateBuildKnownHosts(context.Background(), "my-tmpl")
	require.NoError(t, err)
	assert.Equal(t, "github.com", hosts[0].Hostname)
}

func TestCreateTemplateBuildKnownHost(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/templates/my-tmpl/build_known_hosts", r.URL.Path)
		var body struct {
			BuildKnownHost BuildKnownHostCreateRequest `json:"build_known_host"`
		}
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		assert.Equal(t, "github.com", body.BuildKnownHost.Hostname)
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(BuildKnownHost{Identifier: "kh1", Hostname: body.BuildKnownHost.Hostname})
	}))
	defer srv.Close()

	c := newTestClient(t, srv)
	h, err := c.CreateTemplateBuildKnownHost(context.Background(), "my-tmpl", BuildKnownHostCreateRequest{Hostname: "github.com", PublicKey: "ssh-rsa AAAA"})
	require.NoError(t, err)
	assert.Equal(t, "github.com", h.Hostname)
}

// --- 8. build_languages (UPDATE ONLY; PATCH; :id = package name) ---

func TestUpdateTemplateBuildLanguage(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPatch, r.Method)
		assert.Equal(t, "/templates/my-tmpl/build_languages/ruby", r.URL.Path)
		var body struct {
			BuildEnvironment BuildLanguageUpdateRequest `json:"build_environment"`
		}
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		assert.Equal(t, "3.3.0", body.BuildEnvironment.Version)
		_ = json.NewEncoder(w).Encode(BuildLanguage{Name: "ruby", Version: body.BuildEnvironment.Version})
	}))
	defer srv.Close()

	c := newTestClient(t, srv)
	lang, err := c.UpdateTemplateBuildLanguage(context.Background(), "my-tmpl", "ruby", BuildLanguageUpdateRequest{Version: "3.3.0"})
	require.NoError(t, err)
	assert.Equal(t, "3.3.0", lang.Version)
}

// --- 9. build_configuration (SHOW ONLY; singular route, no :id) ---

func TestGetTemplateBuildConfiguration(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Equal(t, "/templates/my-tmpl/build_configuration", r.URL.Path)
		_ = json.NewEncoder(w).Encode(BuildConfig{Identifier: "be1", Default: true, Packages: map[string]string{"ruby": "3.3.0"}})
	}))
	defer srv.Close()

	c := newTestClient(t, srv)
	cfg, err := c.GetTemplateBuildConfiguration(context.Background(), "my-tmpl")
	require.NoError(t, err)
	assert.True(t, cfg.Default)
	assert.Equal(t, "3.3.0", cfg.Packages["ruby"])
}

// --- 10. servers ---

func TestListTemplateServers(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/templates/my-tmpl/servers", r.URL.Path)
		_ = json.NewEncoder(w).Encode([]Server{{Identifier: "s1", Name: "web", ProtocolType: "ssh"}})
	}))
	defer srv.Close()

	c := newTestClient(t, srv)
	servers, err := c.ListTemplateServers(context.Background(), "my-tmpl", nil)
	require.NoError(t, err)
	assert.Equal(t, "web", servers[0].Name)
}

func TestCreateTemplateServer(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/templates/my-tmpl/servers", r.URL.Path)
		var body struct {
			Server ServerCreateRequest `json:"server"`
		}
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		assert.Equal(t, "ssh", body.Server.ProtocolType)
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(Server{Identifier: "s1", Name: body.Server.Name, ProtocolType: body.Server.ProtocolType})
	}))
	defer srv.Close()

	c := newTestClient(t, srv)
	s, err := c.CreateTemplateServer(context.Background(), "my-tmpl", ServerCreateRequest{Name: "web", ProtocolType: "ssh"})
	require.NoError(t, err)
	assert.Equal(t, "ssh", s.ProtocolType)
}

func TestGetTemplateServer(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/templates/my-tmpl/servers/s1", r.URL.Path)
		_ = json.NewEncoder(w).Encode(Server{Identifier: "s1", Name: "web"})
	}))
	defer srv.Close()

	c := newTestClient(t, srv)
	s, err := c.GetTemplateServer(context.Background(), "my-tmpl", "s1")
	require.NoError(t, err)
	assert.Equal(t, "s1", s.Identifier)
}

func TestDeleteTemplateServer(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodDelete, r.Method)
		assert.Equal(t, "/templates/my-tmpl/servers/s1", r.URL.Path)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	c := newTestClient(t, srv)
	require.NoError(t, c.DeleteTemplateServer(context.Background(), "my-tmpl", "s1"))
}

// --- 11. server_groups (no show; extra fields on create) ---

func TestListTemplateServerGroups(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/templates/my-tmpl/server_groups", r.URL.Path)
		_ = json.NewEncoder(w).Encode([]ServerGroup{{Identifier: "sg1", Name: "Production"}})
	}))
	defer srv.Close()

	c := newTestClient(t, srv)
	groups, err := c.ListTemplateServerGroups(context.Background(), "my-tmpl", nil)
	require.NoError(t, err)
	assert.Equal(t, "Production", groups[0].Name)
}

func TestCreateTemplateServerGroup(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/templates/my-tmpl/server_groups", r.URL.Path)
		var body struct {
			ServerGroup TemplateServerGroupCreateRequest `json:"server_group"`
		}
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		assert.Equal(t, "Staging", body.ServerGroup.Name)
		assert.Equal(t, "production", body.ServerGroup.Environment)
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(ServerGroup{Identifier: "sg-new", Name: body.ServerGroup.Name})
	}))
	defer srv.Close()

	c := newTestClient(t, srv)
	g, err := c.CreateTemplateServerGroup(context.Background(), "my-tmpl", TemplateServerGroupCreateRequest{Name: "Staging", Environment: "production"})
	require.NoError(t, err)
	assert.Equal(t, "Staging", g.Name)
}

func TestUpdateTemplateServerGroup(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPut, r.Method)
		assert.Equal(t, "/templates/my-tmpl/server_groups/sg1", r.URL.Path)
		var body struct {
			ServerGroup TemplateServerGroupUpdateRequest `json:"server_group"`
		}
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		assert.Equal(t, "Renamed", body.ServerGroup.Name)
		_ = json.NewEncoder(w).Encode(ServerGroup{Identifier: "sg1", Name: body.ServerGroup.Name})
	}))
	defer srv.Close()

	c := newTestClient(t, srv)
	g, err := c.UpdateTemplateServerGroup(context.Background(), "my-tmpl", "sg1", TemplateServerGroupUpdateRequest{Name: "Renamed"})
	require.NoError(t, err)
	assert.Equal(t, "Renamed", g.Name)
}

func TestDeleteTemplateServerGroup(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodDelete, r.Method)
		assert.Equal(t, "/templates/my-tmpl/server_groups/sg1", r.URL.Path)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	c := newTestClient(t, srv)
	require.NoError(t, c.DeleteTemplateServerGroup(context.Background(), "my-tmpl", "sg1"))
}
