package commands

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type buildConfigCall struct {
	method string
	path   string
	body   map[string]any
}

// fakeBuildConfigAPI answers the build-configuration and server endpoints
// with the response shapes DeployHQ returns, and records every API call.
// keptServers controls how many of the sent server identifiers the update
// response echoes back (DeployHQ drops identifiers it does not recognise).
type fakeBuildConfigAPI struct {
	mu          sync.Mutex
	calls       []buildConfigCall
	keptServers int // -1 = keep all
	updateFail  bool
}

func (f *fakeBuildConfigAPI) install(t *testing.T) {
	t.Helper()
	orig := http.DefaultTransport
	http.DefaultTransport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		respond := func(status int, body string) (*http.Response, error) {
			return &http.Response{
				StatusCode: status,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
				Request:    r,
			}, nil
		}
		if !strings.Contains(r.URL.Path, "/projects/") {
			return respond(http.StatusOK, `{}`)
		}
		f.mu.Lock()
		defer f.mu.Unlock()
		call := buildConfigCall{method: r.Method, path: r.URL.Path}
		if r.Body != nil {
			raw, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(raw, &call.body)
		}
		f.calls = append(f.calls, call)

		switch {
		case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/servers"):
			return respond(http.StatusOK, `[{"identifier":"aaaaaaaa-1111-2222-3333-444444444444","name":"Production"},{"identifier":"bbbbbbbb-1111-2222-3333-444444444444","name":"Staging"}]`)
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/build_configurations"):
			return respond(http.StatusCreated, `{"identifier":"override-1","packages":{},"default":false,"servers":[]}`)
		case r.Method == http.MethodPut && strings.Contains(r.URL.Path, "/build_configurations/"):
			if f.updateFail {
				return respond(http.StatusUnprocessableEntity, `{"errors":["server Production is already targeted by another server-specific build configuration"]}`)
			}
			ids, _ := call.body["build_environment"].(map[string]any)["server_identifiers"].([]any)
			if f.keptServers >= 0 && f.keptServers < len(ids) {
				ids = ids[:f.keptServers]
			}
			servers := make([]map[string]any, len(ids))
			for i, id := range ids {
				servers[i] = map[string]any{"identifier": id, "name": "srv"}
			}
			out, _ := json.Marshal(map[string]any{"identifier": "override-1", "packages": map[string]any{}, "default": false, "servers": servers})
			return respond(http.StatusOK, string(out))
		}
		return respond(http.StatusNotFound, `{"error":"not found"}`)
	})
	t.Cleanup(func() { http.DefaultTransport = orig })
}

// runBuildConfigsCmd runs the real root command and returns what it wrote to
// stderr. The output envelope writes to the process streams, so they are
// swapped for pipes for the duration of the command.
func runBuildConfigsCmd(t *testing.T, args ...string) (string, error) {
	t.Helper()
	origOut, origErr := os.Stdout, os.Stderr
	outR, outW, err := os.Pipe()
	require.NoError(t, err)
	errR, errW, err := os.Pipe()
	require.NoError(t, err)
	os.Stdout, os.Stderr = outW, errW

	var wg sync.WaitGroup
	var stderr bytes.Buffer
	wg.Add(2)
	go func() { defer wg.Done(); _, _ = io.Copy(io.Discard, outR) }()
	go func() { defer wg.Done(); _, _ = io.Copy(&stderr, errR) }()

	cmd := NewRootCmd("test")
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	cmd.SetArgs(args)
	runErr := cmd.Execute()

	os.Stdout, os.Stderr = origOut, origErr
	_ = outW.Close()
	_ = errW.Close()
	wg.Wait()
	return stderr.String(), runErr
}

func serverIdentifiersSent(t *testing.T, c buildConfigCall) []any {
	t.Helper()
	be, ok := c.body["build_environment"].(map[string]any)
	require.True(t, ok, "update body must wrap build_environment, got %v", c.body)
	ids, ok := be["server_identifiers"].([]any)
	require.True(t, ok, "update body must carry server_identifiers, got %v", be)
	return ids
}

func TestBuildConfigsCreate_WithServerTargetsTheNewOverride(t *testing.T) {
	withResolvableContext(t)
	api := &fakeBuildConfigAPI{keptServers: -1}
	api.install(t)

	_, err := runBuildConfigsCmd(t, "build-configs", "create", "--server", "production", "--json")
	require.NoError(t, err)

	require.Len(t, api.calls, 3, "expected list servers, create, then update")
	assert.Equal(t, http.MethodGet, api.calls[0].method)
	assert.Equal(t, http.MethodPost, api.calls[1].method)
	assert.Equal(t, http.MethodPut, api.calls[2].method)
	assert.True(t, strings.HasSuffix(api.calls[2].path, "/build_configurations/override-1"))
	assert.Equal(t, []any{"aaaaaaaa-1111-2222-3333-444444444444"}, serverIdentifiersSent(t, api.calls[2]))
}

func TestBuildConfigsCreate_WithoutServerWarnsItWillNotApply(t *testing.T) {
	withResolvableContext(t)
	api := &fakeBuildConfigAPI{keptServers: -1}
	api.install(t)

	stderr, err := runBuildConfigsCmd(t, "build-configs", "create")
	require.NoError(t, err)
	require.Len(t, api.calls, 1, "create without --server must not update")
	assert.Contains(t, stderr, "targets no servers")
	assert.Contains(t, stderr, "dhq build-configs update override-1 --server")
}

func TestBuildConfigsCreate_TargetingFailureNamesTheOverride(t *testing.T) {
	withResolvableContext(t)
	api := &fakeBuildConfigAPI{keptServers: -1, updateFail: true}
	api.install(t)

	_, err := runBuildConfigsCmd(t, "build-configs", "create", "--server", "aaaaaaaa-1111-2222-3333-444444444444")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "override-1")
	assert.Contains(t, err.Error(), "already targeted")
}

func TestBuildConfigsUpdate_SendsServerIdentifiers(t *testing.T) {
	withResolvableContext(t)
	api := &fakeBuildConfigAPI{keptServers: -1}
	api.install(t)

	// An identifier passes through without listing servers; a name is resolved.
	_, err := runBuildConfigsCmd(t, "build-configs", "update", "override-1",
		"--server", "bbbbbbbb-1111-2222-3333-444444444444,Production", "--json")
	require.NoError(t, err)

	require.Len(t, api.calls, 2, "expected one server list and one update")
	assert.True(t, strings.HasSuffix(api.calls[0].path, "/servers"))
	assert.Equal(t, http.MethodPut, api.calls[1].method)
	assert.Equal(t,
		[]any{"bbbbbbbb-1111-2222-3333-444444444444", "aaaaaaaa-1111-2222-3333-444444444444"},
		serverIdentifiersSent(t, api.calls[1]))
}

func TestBuildConfigsUpdate_IdentifiersOnlySkipServerLookup(t *testing.T) {
	withResolvableContext(t)
	api := &fakeBuildConfigAPI{keptServers: -1}
	api.install(t)

	_, err := runBuildConfigsCmd(t, "build-configs", "update", "override-1",
		"--server", "bbbbbbbb-1111-2222-3333-444444444444")
	require.NoError(t, err)
	require.Len(t, api.calls, 1)
	assert.Equal(t, http.MethodPut, api.calls[0].method)
}

func TestBuildConfigsUpdate_WarnsWhenServersAreDropped(t *testing.T) {
	withResolvableContext(t)
	api := &fakeBuildConfigAPI{keptServers: 0}
	api.install(t)

	stderr, err := runBuildConfigsCmd(t, "build-configs", "update", "override-1",
		"--server", "cccccccc-1111-2222-3333-444444444444")
	require.NoError(t, err)
	assert.Contains(t, stderr, "targets 0 of the 1 servers")
}

func TestBuildConfigsUpdate_UnknownServerNameFailsWithChoices(t *testing.T) {
	withResolvableContext(t)
	api := &fakeBuildConfigAPI{keptServers: -1}
	api.install(t)

	_, err := runBuildConfigsCmd(t, "build-configs", "update", "override-1", "--server", "nope")
	require.Error(t, err)
	assert.Contains(t, err.Error(), `No unique server matches "nope"`)
	require.Len(t, api.calls, 1, "must not update after a failed lookup")
}

func TestBuildConfigsUpdate_RequiresServerWithoutCallingAPI(t *testing.T) {
	withResolvableContext(t)
	called := blockNetwork(t)

	_, err := runBuildConfigsCmd(t, "build-configs", "update", "override-1")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "--server")
	assert.False(t, *called)
}

func TestBuildConfigs_PackagesFlagRejectedWithoutCallingAPI(t *testing.T) {
	for _, args := range [][]string{
		{"build-configs", "create", "--packages", `{"node":"20"}`},
		{"build-configs", "update", "override-1", "--packages", `{"node":"20"}`},
	} {
		t.Run(args[1], func(t *testing.T) {
			withResolvableContext(t)
			called := blockNetwork(t)
			_, err := runBuildConfigsCmd(t, args...)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "--packages")
			assert.False(t, *called)
		})
	}
}
