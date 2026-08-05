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

// captureCreateBody records the raw JSON body of a POST /projects/:id/servers
// call so tests can assert on exact key placement, not just decoded structs.
func captureCreateBody(t *testing.T, into *map[string]any) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodPost, r.Method)
		require.NoError(t, json.NewDecoder(r.Body).Decode(into))
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(Server{Identifier: "srv-1", Name: "staging"})
	}))
}

// captureUpdateBody does the same for PUT /projects/:id/servers/:id.
func captureUpdateBody(t *testing.T, into *map[string]any) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodPut, r.Method)
		require.NoError(t, json.NewDecoder(r.Body).Decode(into))
		_ = json.NewEncoder(w).Encode(Server{Identifier: "srv-1", Name: "staging"})
	}))
}

func TestCreateServer_ManagedVPS_DeploymentSettingsNestedUnderServer(t *testing.T) {
	// The five deployment settings belong INSIDE `server`; region/size/os_image
	// stay top-level siblings (params[:region], not params[:server][:region]).
	var body map[string]any
	srv := captureCreateBody(t, &body)
	defer srv.Close()

	c := newTestClient(t, srv)
	_, err := c.CreateServer(context.Background(), "my-app", ServerCreateRequest{
		Name:            "staging",
		ProtocolType:    "managed_vps",
		ServerPath:      "/srv/ops-agents",
		Environment:     "staging",
		Branch:          strPtr("staging"),
		AutoDeploy:      boolPtr(false),
		Atomic:          boolPtr(true),
		AtomicStrategy:  "copy_release",
		AtomicRetention: intPtr(5),
		Region:          "lon1",
		Size:            "s-1vcpu-1gb",
		OSImage:         "ubuntu-24-04-x64",
	})
	require.NoError(t, err)

	server, ok := body["server"].(map[string]any)
	require.True(t, ok, "body must wrap the server object")

	assert.Equal(t, "staging", server["branch"])
	assert.Equal(t, false, server["auto_deploy"])
	assert.Equal(t, true, server["atomic"])
	assert.Equal(t, "copy_release", server["atomic_strategy"])
	assert.Equal(t, float64(5), server["atomic_retention"])

	// Provisioning params remain top-level, never nested.
	assert.Equal(t, "lon1", body["region"])
	assert.Equal(t, "s-1vcpu-1gb", body["size"])
	assert.Equal(t, "ubuntu-24-04-x64", body["os_image"])
	for _, k := range []string{"region", "size", "os_image"} {
		_, nested := server[k]
		assert.False(t, nested, "%s must not be nested inside server", k)
	}
}

func TestUpdateServer_SendsDeploymentSettingsWrapped(t *testing.T) {
	var body map[string]any
	srv := captureUpdateBody(t, &body)
	defer srv.Close()

	c := newTestClient(t, srv)
	_, err := c.UpdateServer(context.Background(), "my-app", "srv-1", ServerUpdateRequest{
		Branch:          strPtr("main"),
		AutoDeploy:      boolPtr(true),
		Atomic:          boolPtr(true),
		AtomicStrategy:  "copy_cache",
		AtomicRetention: intPtr(10),
	})
	require.NoError(t, err)

	server, ok := body["server"].(map[string]any)
	require.True(t, ok, "body must wrap the server object")
	assert.Equal(t, "main", server["branch"])
	assert.Equal(t, true, server["auto_deploy"])
	assert.Equal(t, true, server["atomic"])
	assert.Equal(t, "copy_cache", server["atomic_strategy"])
	assert.Equal(t, float64(10), server["atomic_retention"])
}

func TestCreateServer_ExplicitFalseBooleansAreSent(t *testing.T) {
	// omitempty on a bool would drop `false`; pointers keep the distinction
	// between "not supplied" and "explicitly false".
	var body map[string]any
	srv := captureCreateBody(t, &body)
	defer srv.Close()

	c := newTestClient(t, srv)
	_, err := c.CreateServer(context.Background(), "my-app", ServerCreateRequest{
		Name:         "staging",
		ProtocolType: "ssh",
		AutoDeploy:   boolPtr(false),
		Atomic:       boolPtr(false),
	})
	require.NoError(t, err)

	server := body["server"].(map[string]any)
	require.Contains(t, server, "auto_deploy", "explicit false must survive serialisation")
	require.Contains(t, server, "atomic", "explicit false must survive serialisation")
	assert.Equal(t, false, server["auto_deploy"])
	assert.Equal(t, false, server["atomic"])
}

func TestUpdateServer_ExplicitFalseBooleansAreSent(t *testing.T) {
	var body map[string]any
	srv := captureUpdateBody(t, &body)
	defer srv.Close()

	c := newTestClient(t, srv)
	_, err := c.UpdateServer(context.Background(), "my-app", "srv-1", ServerUpdateRequest{
		AutoDeploy: boolPtr(false),
		Atomic:     boolPtr(false),
	})
	require.NoError(t, err)

	server := body["server"].(map[string]any)
	require.Contains(t, server, "auto_deploy")
	require.Contains(t, server, "atomic")
	assert.Equal(t, false, server["auto_deploy"])
	assert.Equal(t, false, server["atomic"])
}

func TestCreateServer_OmittedDeploymentSettingsAreAbsent(t *testing.T) {
	// An unset field must not reach the wire — a zero-value atomic_retention
	// would be rejected by the backend's `greater_than_or_equal_to: 1`.
	var body map[string]any
	srv := captureCreateBody(t, &body)
	defer srv.Close()

	c := newTestClient(t, srv)
	_, err := c.CreateServer(context.Background(), "my-app", ServerCreateRequest{
		Name: "staging", ProtocolType: "ssh",
	})
	require.NoError(t, err)

	server := body["server"].(map[string]any)
	for _, k := range []string{"branch", "auto_deploy", "atomic", "atomic_strategy", "atomic_retention"} {
		assert.NotContains(t, server, k, "%s must be omitted when not supplied", k)
	}
}

func TestUpdateServer_OmittedDeploymentSettingsAreAbsent(t *testing.T) {
	var body map[string]any
	srv := captureUpdateBody(t, &body)
	defer srv.Close()

	c := newTestClient(t, srv)
	_, err := c.UpdateServer(context.Background(), "my-app", "srv-1", ServerUpdateRequest{Name: "renamed"})
	require.NoError(t, err)

	server := body["server"].(map[string]any)
	assert.Equal(t, "renamed", server["name"])
	for _, k := range []string{"branch", "auto_deploy", "atomic", "atomic_strategy", "atomic_retention"} {
		assert.NotContains(t, server, k, "%s must be omitted when not supplied", k)
	}
}

func intPtr(i int) *int { return &i }
