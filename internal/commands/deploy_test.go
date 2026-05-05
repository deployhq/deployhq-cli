package commands

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/deployhq/deployhq-cli/pkg/sdk"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDeployDryRunFlag(t *testing.T) {
	cmd := NewRootCmd("test")
	deployCmd, _, _ := cmd.Find([]string{"deploy"})
	assert.NotNil(t, deployCmd)
	assert.NotNil(t, deployCmd.Flags().Lookup("dry-run"))
}

func TestDeployDryRunMutuallyExclusiveWithWait(t *testing.T) {
	cmd := NewRootCmd("test")
	cmd.SetArgs([]string{"deploy", "--dry-run", "--wait", "-p", "test-project"})
	err := cmd.Execute()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "mutually exclusive")
}

// newTestSDKClient returns an SDK client wired to the given httptest server.
func newTestSDKClient(t *testing.T, server *httptest.Server) *sdk.Client {
	t.Helper()
	c, err := sdk.New("test", "u@e.com", "k",
		sdk.WithBaseURL(server.URL),
		sdk.WithHTTPClient(server.Client()),
	)
	require.NoError(t, err)
	return c
}

// branchEndpointMux serves /branches and /servers/:id and /latest_revision.
// Tracks which endpoints were called so tests can verify the resolution order.
type branchEndpointMux struct {
	branches            map[string]string
	servers             map[string]sdk.Server
	latestRevision      string
	branchesCalled      bool
	latestRevCalled     bool
	getServerCalledWith string
}

func (m *branchEndpointMux) handler(t *testing.T) http.HandlerFunc {
	t.Helper()
	return func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/projects/p/repository/branches":
			m.branchesCalled = true
			_ = json.NewEncoder(w).Encode(m.branches)
		case r.Method == http.MethodGet && r.URL.Path == "/projects/p/repository/latest_revision":
			m.latestRevCalled = true
			_ = json.NewEncoder(w).Encode(map[string]string{"ref": m.latestRevision})
		case r.Method == http.MethodGet && len(r.URL.Path) > len("/projects/p/servers/") &&
			r.URL.Path[:len("/projects/p/servers/")] == "/projects/p/servers/":
			id := r.URL.Path[len("/projects/p/servers/"):]
			m.getServerCalledWith = id
			s, ok := m.servers[id]
			if !ok {
				w.WriteHeader(http.StatusNotFound)
				_, _ = w.Write([]byte(`{"error":"not found"}`))
				return
			}
			_ = json.NewEncoder(w).Encode(s)
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusInternalServerError)
		}
	}
}

func TestResolveBranchAndRevision_UsesServerPreferredBranch(t *testing.T) {
	// Bug fix: when --branch is not specified, the server's preferred_branch
	// must be used (not the repository's default branch).
	mux := &branchEndpointMux{
		branches: map[string]string{
			"main":    "sha-of-main",
			"develop": "sha-of-develop",
		},
		latestRevision: "sha-of-main", // would be used if branch resolution is broken
	}
	srv := httptest.NewServer(mux.handler(t))
	defer srv.Close()
	client := newTestSDKClient(t, srv)

	knownServer := &sdk.Server{
		Identifier:      "srv-1",
		PreferredBranch: "develop",
	}

	branch, revision, err := resolveBranchAndRevision(
		t.Context(), client, "p", "srv-1", "", "", knownServer,
	)
	require.NoError(t, err)
	assert.Equal(t, "develop", branch, "should use server's preferred_branch")
	assert.Equal(t, "sha-of-develop", revision, "should use develop's tip, not main's")
	assert.True(t, mux.branchesCalled, "should query /branches to map branch→sha")
	assert.False(t, mux.latestRevCalled, "should NOT fall back to repo default")
}

func TestResolveBranchAndRevision_FlagBranchOverridesServerPreferred(t *testing.T) {
	// Bug fix: when --branch is specified, its tip SHA must be used —
	// previously end_revision came from the repo default branch and silently
	// overrode the user's --branch on the API.
	mux := &branchEndpointMux{
		branches: map[string]string{
			"main":      "sha-of-main",
			"feature-x": "sha-of-feature",
		},
		latestRevision: "sha-of-main",
	}
	srv := httptest.NewServer(mux.handler(t))
	defer srv.Close()
	client := newTestSDKClient(t, srv)

	knownServer := &sdk.Server{
		Identifier:      "srv-1",
		PreferredBranch: "main",
	}

	branch, revision, err := resolveBranchAndRevision(
		t.Context(), client, "p", "srv-1", "feature-x", "", knownServer,
	)
	require.NoError(t, err)
	assert.Equal(t, "feature-x", branch)
	assert.Equal(t, "sha-of-feature", revision, "must be feature-x's tip, not main's")
	assert.False(t, mux.latestRevCalled)
}

func TestResolveBranchAndRevision_FetchesServerWhenNotKnown(t *testing.T) {
	// When the caller didn't already resolve the Server (e.g. user passed a
	// raw UUID), we should fetch it via GetServer to read preferred_branch.
	mux := &branchEndpointMux{
		branches: map[string]string{"prod-branch": "sha-of-prod"},
		servers: map[string]sdk.Server{
			"srv-uuid": {Identifier: "srv-uuid", PreferredBranch: "prod-branch"},
		},
	}
	srv := httptest.NewServer(mux.handler(t))
	defer srv.Close()
	client := newTestSDKClient(t, srv)

	branch, revision, err := resolveBranchAndRevision(
		t.Context(), client, "p", "srv-uuid", "", "", nil, // knownServer = nil
	)
	require.NoError(t, err)
	assert.Equal(t, "prod-branch", branch)
	assert.Equal(t, "sha-of-prod", revision)
	assert.Equal(t, "srv-uuid", mux.getServerCalledWith)
}

func TestResolveBranchAndRevision_ServerGroupFallsBackGracefully(t *testing.T) {
	// Server-group identifiers cause GetServer to 404. We should swallow that
	// and fall back to the repository default branch's tip.
	mux := &branchEndpointMux{
		servers:        map[string]sdk.Server{}, // any GetServer → 404
		latestRevision: "sha-of-default",
	}
	srv := httptest.NewServer(mux.handler(t))
	defer srv.Close()
	client := newTestSDKClient(t, srv)

	branch, revision, err := resolveBranchAndRevision(
		t.Context(), client, "p", "group-id", "", "", nil,
	)
	require.NoError(t, err)
	assert.Equal(t, "", branch, "no branch when server-group has no preferred branch")
	assert.Equal(t, "sha-of-default", revision)
	assert.True(t, mux.latestRevCalled)
}

func TestResolveBranchAndRevision_ExplicitRevisionRespected(t *testing.T) {
	// When --revision is explicit, no API calls should be needed and the
	// caller's SHA must pass through unchanged.
	mux := &branchEndpointMux{}
	srv := httptest.NewServer(mux.handler(t))
	defer srv.Close()
	client := newTestSDKClient(t, srv)

	knownServer := &sdk.Server{Identifier: "srv-1", PreferredBranch: "main"}

	branch, revision, err := resolveBranchAndRevision(
		t.Context(), client, "p", "srv-1", "feature-x", "explicit-sha", knownServer,
	)
	require.NoError(t, err)
	assert.Equal(t, "feature-x", branch)
	assert.Equal(t, "explicit-sha", revision)
	assert.False(t, mux.branchesCalled)
	assert.False(t, mux.latestRevCalled)
}

func TestResolveBranchAndRevision_UnknownBranchErrors(t *testing.T) {
	// If --branch points at a branch that doesn't exist, we must error out
	// rather than silently deploy the repo default's tip.
	mux := &branchEndpointMux{
		branches: map[string]string{"main": "sha-of-main"},
	}
	srv := httptest.NewServer(mux.handler(t))
	defer srv.Close()
	client := newTestSDKClient(t, srv)

	_, _, err := resolveBranchAndRevision(
		t.Context(), client, "p", "", "nope", "", nil,
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), `"nope"`)
	assert.Contains(t, err.Error(), "not found")
}

func TestResolveBranchAndRevision_FallsBackWhenNoBranchAvailable(t *testing.T) {
	// No --branch, no server, no preferred_branch → use repo default revision.
	mux := &branchEndpointMux{latestRevision: "sha-of-default"}
	srv := httptest.NewServer(mux.handler(t))
	defer srv.Close()
	client := newTestSDKClient(t, srv)

	branch, revision, err := resolveBranchAndRevision(
		t.Context(), client, "p", "", "", "", nil,
	)
	require.NoError(t, err)
	assert.Equal(t, "", branch)
	assert.Equal(t, "sha-of-default", revision)
	assert.True(t, mux.latestRevCalled)
}

func TestResolveBranchAndRevision_PreferredBranchEmptyFallsToBranchField(t *testing.T) {
	// Some servers populate Branch but not PreferredBranch. Treat both as the
	// same source of truth.
	mux := &branchEndpointMux{
		branches: map[string]string{"legacy": "sha-of-legacy"},
	}
	srv := httptest.NewServer(mux.handler(t))
	defer srv.Close()
	client := newTestSDKClient(t, srv)

	knownServer := &sdk.Server{
		Identifier: "srv-1",
		Branch:     "legacy", // PreferredBranch left empty
	}
	branch, revision, err := resolveBranchAndRevision(
		t.Context(), client, "p", "srv-1", "", "", knownServer,
	)
	require.NoError(t, err)
	assert.Equal(t, "legacy", branch)
	assert.Equal(t, "sha-of-legacy", revision)
}
