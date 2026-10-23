package commands

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/deployhq/deployhq-cli/internal/detect"
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
	assert.False(t, api.listedServers(), "an identifier needs no lookup")
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
