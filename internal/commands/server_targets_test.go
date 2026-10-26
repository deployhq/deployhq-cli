package commands

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/deployhq/deployhq-cli/internal/detect"
	"github.com/deployhq/deployhq-cli/internal/output"
	"github.com/deployhq/deployhq-cli/pkg/sdk"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	targetServerUUID = "8e1c7a52-1111-4c3e-9a0b-000000000001"
	otherServerUUID  = "8e1c7a52-2222-4c3e-9a0b-000000000002"
	targetGroupUUID  = "8e1c7a52-3333-4c3e-9a0b-000000000003"
)

// targetingAPI is a path-routed fake of the DeployHQ API: it answers server and
// server-group listings so name resolution can run, and records every write so
// tests can assert the exact server-targeting keys a command put on the wire.
type targetingAPI struct {
	mu      sync.Mutex
	gets    []string
	writes  []string
	bodies  []map[string]any
	created string // JSON returned for writes
}

func installTargetingAPI(t *testing.T, created string) *targetingAPI {
	t.Helper()
	api := &targetingAPI{created: created}
	orig := http.DefaultTransport
	http.DefaultTransport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		api.mu.Lock()
		defer api.mu.Unlock()
		respond := func(status int, body string) (*http.Response, error) {
			return &http.Response{
				StatusCode: status,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
				Request:    r,
			}, nil
		}
		// The root command's update check talks to another host; ignore it.
		if !strings.Contains(r.URL.Path, "/projects/") && !strings.Contains(r.URL.Path, "/templates/") {
			return respond(http.StatusOK, `{}`)
		}
		if r.Method == http.MethodGet {
			api.gets = append(api.gets, r.URL.Path)
			switch {
			case strings.HasSuffix(r.URL.Path, "/servers"):
				return respond(http.StatusOK, `[
					{"identifier":"`+targetServerUUID+`","name":"Production","protocol_type":"ssh"},
					{"identifier":"`+otherServerUUID+`","name":"Staging","protocol_type":"ssh"}]`)
			case strings.HasSuffix(r.URL.Path, "/server_groups"):
				return respond(http.StatusOK, `[{"identifier":"`+targetGroupUUID+`","name":"Web Cluster"}]`)
			}
			return respond(http.StatusOK, `[]`)
		}
		api.writes = append(api.writes, r.Method+" "+r.URL.Path)
		var body map[string]any
		if r.Body != nil {
			raw, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(raw, &body)
		}
		api.bodies = append(api.bodies, body)
		return respond(http.StatusCreated, api.created)
	})
	t.Cleanup(func() { http.DefaultTransport = orig })
	return api
}

// payload returns the single write's resource object (e.g. body["excluded_file"]).
func (a *targetingAPI) payload(t *testing.T, key string) map[string]any {
	t.Helper()
	a.mu.Lock()
	defer a.mu.Unlock()
	require.Len(t, a.writes, 1, "expected exactly one write request")
	obj, ok := a.bodies[0][key].(map[string]any)
	require.True(t, ok, "request body must wrap a %q object, got: %v", key, a.bodies[0])
	return obj
}

func (a *targetingAPI) writeCount() int {
	a.mu.Lock()
	defer a.mu.Unlock()
	return len(a.writes)
}

// getCount is how many GET requests hit a path ending in suffix.
func (a *targetingAPI) getCount(suffix string) int {
	a.mu.Lock()
	defer a.mu.Unlock()
	n := 0
	for _, p := range a.gets {
		if strings.HasSuffix(p, suffix) {
			n++
		}
	}
	return n
}

func (a *targetingAPI) listedServers() bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	for _, p := range a.gets {
		if strings.HasSuffix(p, "/servers") {
			return true
		}
	}
	return false
}

// assertAllServers checks the create defaulted to every server.
func assertAllServers(t *testing.T, obj map[string]any) {
	t.Helper()
	assert.Equal(t, true, obj["all_servers"], "create must target all servers by default")
	assert.NotContains(t, obj, "server_identifiers")
}

// assertTargets checks the request targets exactly ids and does not also send
// all_servers (which would override the list).
func assertTargets(t *testing.T, obj map[string]any, ids ...string) {
	t.Helper()
	assert.NotContains(t, obj, "all_servers")
	want := make([]any, len(ids))
	for i, id := range ids {
		want[i] = id
	}
	assert.Equal(t, want, obj["server_identifiers"])
}

// assertTargetingUntouched checks an update leaves server targeting alone.
func assertTargetingUntouched(t *testing.T, obj map[string]any) {
	t.Helper()
	assert.NotContains(t, obj, "all_servers")
	assert.NotContains(t, obj, "server_identifiers")
}

// ── excluded files ────────────────────────────────────────────────────────────

const excludedFileJSON = `{"identifier":"ef-1","path":"deploy.sh","servers":[]}`

func TestExcludedFilesCreate_DefaultsToAllServers(t *testing.T) {
	withResolvableContext(t)
	api := installTargetingAPI(t, excludedFileJSON)

	require.NoError(t, runServersCmd(t, "excluded-files", "create", "--path", "deploy.sh"))

	obj := api.payload(t, "excluded_file")
	assert.Equal(t, "deploy.sh", obj["path"])
	assertAllServers(t, obj)
	assert.False(t, api.listedServers(), "default create must not need a server listing")
}

func TestExcludedFilesCreate_ServerIdentifierPassesThrough(t *testing.T) {
	withResolvableContext(t)
	api := installTargetingAPI(t, excludedFileJSON)

	require.NoError(t, runServersCmd(t, "excluded-files", "create", "--path", "deploy.sh", "--server", targetServerUUID))

	assertTargets(t, api.payload(t, "excluded_file"), targetServerUUID)
	// A long name with a dash can look like an identifier, so even an
	// identifier is checked against the names — with one listing of each.
	assert.Equal(t, 1, api.getCount("/servers"))
	assert.Equal(t, 1, api.getCount("/server_groups"))
}

func TestExcludedFilesCreate_ResolvesServerAndGroupNames(t *testing.T) {
	withResolvableContext(t)
	api := installTargetingAPI(t, excludedFileJSON)

	require.NoError(t, runServersCmd(t, "excluded-files", "create", "--path", "deploy.sh",
		"--server", "production", "--server", "Web Cluster"))

	assertTargets(t, api.payload(t, "excluded_file"), targetServerUUID, targetGroupUUID)
}

func TestExcludedFilesCreate_UnknownServerFailsWithoutWriting(t *testing.T) {
	withResolvableContext(t)
	api := installTargetingAPI(t, excludedFileJSON)

	err := runServersCmd(t, "excluded-files", "create", "--path", "deploy.sh", "--server", "nope")
	require.Error(t, err)
	assert.Contains(t, err.Error(), `"nope"`)
	assert.Equal(t, 0, api.writeCount(), "an unresolved server must not create a rule")
}

func TestExcludedFilesUpdate_PathOnlyLeavesTargetingUntouched(t *testing.T) {
	withResolvableContext(t)
	api := installTargetingAPI(t, excludedFileJSON)

	require.NoError(t, runServersCmd(t, "excluded-files", "update", "ef-1", "--path", "*.log"))

	obj := api.payload(t, "excluded_file")
	assert.Equal(t, "*.log", obj["path"])
	assertTargetingUntouched(t, obj)
}

func TestExcludedFilesUpdate_AllServersWithoutPath(t *testing.T) {
	withResolvableContext(t)
	api := installTargetingAPI(t, excludedFileJSON)

	require.NoError(t, runServersCmd(t, "excluded-files", "update", "ef-1", "--all-servers"))

	obj := api.payload(t, "excluded_file")
	assert.NotContains(t, obj, "path", "an omitted --path must not be sent as empty")
	assertAllServers(t, obj)
}

func TestExcludedFilesUpdate_RejectsServerWithAllServers(t *testing.T) {
	withResolvableContext(t)
	api := installTargetingAPI(t, excludedFileJSON)

	err := runServersCmd(t, "excluded-files", "update", "ef-1", "--all-servers", "--server", targetServerUUID)
	require.Error(t, err)
	assert.Equal(t, 0, api.writeCount())
}

func TestExcludedFilesUpdate_RequiresSomethingToChange(t *testing.T) {
	withResolvableContext(t)
	api := installTargetingAPI(t, excludedFileJSON)

	require.Error(t, runServersCmd(t, "excluded-files", "update", "ef-1"))
	assert.Equal(t, 0, api.writeCount())
}

func TestLaunchStaticExtras_ExcludedFilesApplyToAllServers(t *testing.T) {
	var bodies []map[string]any
	var mu sync.Mutex
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodGet {
			_, _ = w.Write([]byte(`[]`))
			return
		}
		if strings.HasSuffix(r.URL.Path, "/excluded_files") {
			var body map[string]any
			_ = json.NewDecoder(r.Body).Decode(&body)
			mu.Lock()
			bodies = append(bodies, body)
			mu.Unlock()
		}
		_, _ = w.Write([]byte(excludedFileJSON))
	}))
	defer srv.Close()

	env, _, _ := testLaunchEnvelope()
	launchApplyStaticExtras(context.Background(), env, launchConfig{projectID: "my-app"}, newTestClient(t, srv),
		detect.Result{ExcludedFiles: []string{"node_modules"}})

	mu.Lock()
	defer mu.Unlock()
	require.Len(t, bodies, 1)
	obj, ok := bodies[0]["excluded_file"].(map[string]any)
	require.True(t, ok)
	assertAllServers(t, obj)
}

// ── config files ──────────────────────────────────────────────────────────────

const configFileJSON = `{"identifier":"cf-1","path":".env","body":"A=1"}`

func TestConfigFilesCreate_DefaultsToAllServers(t *testing.T) {
	withResolvableContext(t)
	api := installTargetingAPI(t, configFileJSON)

	require.NoError(t, runServersCmd(t, "config-files", "create", "--path", ".env", "--body", "A=1"))

	assertAllServers(t, api.payload(t, "config_file"))
}

func TestConfigFilesCreate_ServerNameResolves(t *testing.T) {
	withResolvableContext(t)
	api := installTargetingAPI(t, configFileJSON)

	require.NoError(t, runServersCmd(t, "config-files", "create", "--path", ".env", "--body", "A=1", "--server", "Staging"))

	assertTargets(t, api.payload(t, "config_file"), otherServerUUID)
}

func TestConfigFilesCreate_BuildOnlySendsNoTargeting(t *testing.T) {
	withResolvableContext(t)
	api := installTargetingAPI(t, configFileJSON)

	require.NoError(t, runServersCmd(t, "config-files", "create", "--path", ".npmrc", "--body", "x", "--build-only"))

	obj := api.payload(t, "config_file")
	assertTargetingUntouched(t, obj)
	assert.Equal(t, true, obj["build"])
}

func TestConfigFilesCreate_BuildOnlyConflictsWithServer(t *testing.T) {
	withResolvableContext(t)
	api := installTargetingAPI(t, configFileJSON)

	require.Error(t, runServersCmd(t, "config-files", "create", "--path", ".npmrc", "--body", "x",
		"--build-only", "--server", targetServerUUID))
	assert.Equal(t, 0, api.writeCount())
}

func TestConfigFilesUpdate_BodyOnlyLeavesTargetingUntouched(t *testing.T) {
	withResolvableContext(t)
	api := installTargetingAPI(t, configFileJSON)

	require.NoError(t, runServersCmd(t, "config-files", "update", "cf-1", "--body", "B=2"))

	assertTargetingUntouched(t, api.payload(t, "config_file"))
}

func TestConfigFilesUpdate_AllServers(t *testing.T) {
	withResolvableContext(t)
	api := installTargetingAPI(t, configFileJSON)

	require.NoError(t, runServersCmd(t, "config-files", "update", "cf-1", "--all-servers"))

	assertAllServers(t, api.payload(t, "config_file"))
}

// ── SSH commands ──────────────────────────────────────────────────────────────

const sshCommandJSON = `{"identifier":"cmd-1","description":"Restart","command":"touch tmp/restart.txt"}`

func TestSSHCommandsCreate_DefaultsToAllServers(t *testing.T) {
	withResolvableContext(t)
	api := installTargetingAPI(t, sshCommandJSON)

	require.NoError(t, runServersCmd(t, "ssh-commands", "create", "--command", "touch tmp/restart.txt"))

	assertAllServers(t, api.payload(t, "command"))
}

func TestSSHCommandsCreate_ServerTargets(t *testing.T) {
	withResolvableContext(t)
	api := installTargetingAPI(t, sshCommandJSON)

	require.NoError(t, runServersCmd(t, "ssh-commands", "create", "--command", "x", "--server", "production,"+otherServerUUID))

	assertTargets(t, api.payload(t, "command"), targetServerUUID, otherServerUUID)
}

func TestSSHCommandsUpdate_LeavesTargetingUntouched(t *testing.T) {
	withResolvableContext(t)
	api := installTargetingAPI(t, sshCommandJSON)

	require.NoError(t, runServersCmd(t, "ssh-commands", "update", "cmd-1", "--description", "Restart app"))

	assertTargetingUntouched(t, api.payload(t, "command"))
}

// ── integrations ──────────────────────────────────────────────────────────────

const integrationJSON = `{"identifier":"ig-1","hook_type":"discord","name":"Discord"}`

func TestIntegrationsCreate_DefaultsToAllServers(t *testing.T) {
	withResolvableContext(t)
	api := installTargetingAPI(t, integrationJSON)

	require.NoError(t, runServersCmd(t, "integrations", "create", "--type", "discord"))

	assertAllServers(t, api.payload(t, "integration"))
}

func TestIntegrationsCreate_ServerTargets(t *testing.T) {
	withResolvableContext(t)
	api := installTargetingAPI(t, integrationJSON)

	require.NoError(t, runServersCmd(t, "integrations", "create", "--type", "discord", "--server", targetGroupUUID))

	assertTargets(t, api.payload(t, "integration"), targetGroupUUID)
}

func TestIntegrationsUpdate_NameOnlyLeavesTargetingUntouched(t *testing.T) {
	withResolvableContext(t)
	api := installTargetingAPI(t, integrationJSON)

	require.NoError(t, runServersCmd(t, "integrations", "update", "ig-1", "--name", "Deploys"))

	assertTargetingUntouched(t, api.payload(t, "integration"))
}

// ── templates ─────────────────────────────────────────────────────────────────

func TestTemplateCreates_DefaultToAllServers(t *testing.T) {
	cases := []struct {
		name string
		args []string
		key  string
		resp string
	}{
		{"excluded file", []string{"templates", "excluded-files", "create", "--template", "tpl", "--path", "deploy.sh"}, "excluded_file", excludedFileJSON},
		{"config file", []string{"templates", "config-files", "create", "--template", "tpl", "--path", ".env", "--body", "A=1"}, "config_file", configFileJSON},
		{"ssh command", []string{"templates", "ssh-commands", "create", "--template", "tpl", "--command", "x"}, "command", sshCommandJSON},
		{"integration", []string{"templates", "integrations", "create", "--template", "tpl", "--hook-type", "discord"}, "integration", integrationJSON},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			withResolvableContext(t)
			api := installTargetingAPI(t, tc.resp)

			require.NoError(t, runServersCmd(t, tc.args...))

			assertAllServers(t, api.payload(t, tc.key))
		})
	}
}

// ── resolveServerTargets ──────────────────────────────────────────────────────

const duplicateServerUUID = "8e1c7a52-4444-4c3e-9a0b-000000000004"

// targetsServer answers the server and server-group listings with the given
// status and body, so resolveServerTargets can be exercised directly.
func targetsServer(t *testing.T, serversStatus int, servers string, groupsStatus int, groups string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.HasSuffix(r.URL.Path, "/servers"):
			w.WriteHeader(serversStatus)
			_, _ = w.Write([]byte(servers))
		case strings.HasSuffix(r.URL.Path, "/server_groups"):
			w.WriteHeader(groupsStatus)
			_, _ = w.Write([]byte(groups))
		default:
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"error":"not found"}`))
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestResolveServerTargets_DuplicateServerNameIsAmbiguous(t *testing.T) {
	srv := targetsServer(t, http.StatusOK, `[
		{"identifier":"`+targetServerUUID+`","name":"Production","protocol_type":"ssh"},
		{"identifier":"`+duplicateServerUUID+`","name":"Production","protocol_type":"ssh"},
		{"identifier":"`+otherServerUUID+`","name":"Staging","protocol_type":"ssh"}]`,
		http.StatusOK, `[]`)

	ids, err := resolveServerTargets(context.Background(), newTestClient(t, srv), "my-app", []string{"production"})
	require.Error(t, err)
	assert.Nil(t, ids)

	var userErr *output.UserError
	require.ErrorAs(t, err, &userErr)
	assert.Contains(t, userErr.Message, `"production"`)
	assert.Contains(t, userErr.Message, "more than one server")
	assert.Contains(t, userErr.Hint, targetServerUUID)
	assert.Contains(t, userErr.Hint, duplicateServerUUID)
	assert.NotContains(t, userErr.Hint, otherServerUUID, "only the matching servers are candidates")
}

func TestResolveServerTargets_DuplicateGroupNameIsAmbiguous(t *testing.T) {
	const dupGroupUUID = "8e1c7a52-5555-4c3e-9a0b-000000000005"
	srv := targetsServer(t, http.StatusOK, `[
		{"identifier":"`+targetServerUUID+`","name":"Production","protocol_type":"ssh"}]`,
		http.StatusOK, `[
		{"identifier":"`+targetGroupUUID+`","name":"Web Cluster"},
		{"identifier":"`+dupGroupUUID+`","name":"web cluster"}]`)

	_, err := resolveServerTargets(context.Background(), newTestClient(t, srv), "my-app", []string{"Web Cluster"})
	require.Error(t, err)

	var userErr *output.UserError
	require.ErrorAs(t, err, &userErr)
	assert.Contains(t, userErr.Message, `"Web Cluster"`)
	assert.Contains(t, userErr.Message, "more than one server group")
	assert.Contains(t, userErr.Hint, targetGroupUUID)
	assert.Contains(t, userErr.Hint, dupGroupUUID)
}

func TestResolveServerTargets_FailedGroupListIsReportedForNonServerName(t *testing.T) {
	srv := targetsServer(t, http.StatusOK, `[
		{"identifier":"`+targetServerUUID+`","name":"Production","protocol_type":"ssh"}]`,
		http.StatusInternalServerError, `{"error":"boom"}`)

	_, err := resolveServerTargets(context.Background(), newTestClient(t, srv), "my-app", []string{"Web Cluster"})
	require.Error(t, err)

	var apiErr *sdk.APIError
	require.ErrorAs(t, err, &apiErr, "the group-list failure must surface, not an unknown-name error")
	assert.Equal(t, http.StatusInternalServerError, apiErr.StatusCode)
	var userErr *output.UserError
	assert.False(t, errors.As(err, &userErr), "must not be reported as an unknown server")
	assert.Contains(t, err.Error(), `"Web Cluster"`)
}

func TestResolveServerTargets_FailedGroupListStillResolvesServerName(t *testing.T) {
	srv := targetsServer(t, http.StatusOK, `[
		{"identifier":"`+targetServerUUID+`","name":"Production","protocol_type":"ssh"}]`,
		http.StatusInternalServerError, `{"error":"boom"}`)

	ids, err := resolveServerTargets(context.Background(), newTestClient(t, srv), "my-app", []string{"Production"})
	require.NoError(t, err)
	assert.Equal(t, []string{targetServerUUID}, ids)
}

// A name of 32+ characters containing a dash looks like an identifier to
// isUUID, so names must be looked up before a value is taken as an identifier.
const (
	longServerName = "production-europe-west-web-server"
	longGroupName  = "production-europe-west-web-cluster"
)

func longNamesServer(t *testing.T, groupsStatus int) *httptest.Server {
	t.Helper()
	return targetsServer(t, http.StatusOK, `[
		{"identifier":"`+targetServerUUID+`","name":"`+longServerName+`","protocol_type":"ssh"},
		{"identifier":"`+otherServerUUID+`","name":"Staging","protocol_type":"ssh"}]`,
		groupsStatus, `[{"identifier":"`+targetGroupUUID+`","name":"`+longGroupName+`"}]`)
}

func TestResolveServerTargets_LongDashedServerNameResolves(t *testing.T) {
	require.True(t, isUUID(longServerName), "fixture must look like an identifier")
	srv := longNamesServer(t, http.StatusOK)

	ids, err := resolveServerTargets(context.Background(), newTestClient(t, srv), "my-app", []string{longServerName})
	require.NoError(t, err)
	assert.Equal(t, []string{targetServerUUID}, ids)
}

func TestResolveServerTargets_LongDashedGroupNameResolves(t *testing.T) {
	require.True(t, isUUID(longGroupName), "fixture must look like an identifier")
	srv := longNamesServer(t, http.StatusOK)

	ids, err := resolveServerTargets(context.Background(), newTestClient(t, srv), "my-app", []string{longGroupName})
	require.NoError(t, err)
	assert.Equal(t, []string{targetGroupUUID}, ids)
}

func TestResolveServerTargets_ListedIdentifiersResolveToThemselves(t *testing.T) {
	srv := longNamesServer(t, http.StatusOK)

	ids, err := resolveServerTargets(context.Background(), newTestClient(t, srv), "my-app",
		[]string{otherServerUUID, targetGroupUUID})
	require.NoError(t, err)
	assert.Equal(t, []string{otherServerUUID, targetGroupUUID}, ids)
}

func TestResolveServerTargets_UnlistedIdentifierPassesThrough(t *testing.T) {
	const unlisted = "8e1c7a52-9999-4c3e-9a0b-000000000009"
	srv := longNamesServer(t, http.StatusOK)

	ids, err := resolveServerTargets(context.Background(), newTestClient(t, srv), "my-app", []string{unlisted})
	require.NoError(t, err)
	assert.Equal(t, []string{unlisted}, ids)
}

func TestResolveServerTargets_IdentifierPassesThroughWhenGroupListFails(t *testing.T) {
	const unlisted = "8e1c7a52-9999-4c3e-9a0b-000000000009"
	srv := longNamesServer(t, http.StatusInternalServerError)

	ids, err := resolveServerTargets(context.Background(), newTestClient(t, srv), "my-app", []string{unlisted})
	require.NoError(t, err)
	assert.Equal(t, []string{unlisted}, ids)
}

func TestResolveServerTargets_IdentifierPassesThroughWhenServerListFails(t *testing.T) {
	srv := targetsServer(t, http.StatusInternalServerError, `{"error":"boom"}`, http.StatusOK, `[]`)

	ids, err := resolveServerTargets(context.Background(), newTestClient(t, srv), "my-app", []string{targetServerUUID})
	require.NoError(t, err)
	assert.Equal(t, []string{targetServerUUID}, ids)

	_, err = resolveServerTargets(context.Background(), newTestClient(t, srv), "my-app", []string{"Production"})
	var apiErr *sdk.APIError
	require.ErrorAs(t, err, &apiErr, "a name cannot be resolved without the server list")
}
