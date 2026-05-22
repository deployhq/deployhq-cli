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

func TestResolveStartRevision_DefaultsToServerLastRevision(t *testing.T) {
	// Issue #5: dhq deploy was always doing a full-branch deploy because it
	// never populated start_revision. The default for an incremental deploy is
	// the server's last successfully deployed commit.
	srv := &sdk.Server{Identifier: "srv-1", LastRevision: "last-deploy-sha"}
	got := resolveStartRevision(srv, "", false)
	assert.Equal(t, "last-deploy-sha", got)
}

func TestResolveStartRevision_FlagOverridesServerLastRevision(t *testing.T) {
	// --start-revision is the explicit override path (e.g. for hotfixes
	// starting from a specific commit other than the last deploy).
	srv := &sdk.Server{Identifier: "srv-1", LastRevision: "last-deploy-sha"}
	got := resolveStartRevision(srv, "explicit-start", false)
	assert.Equal(t, "explicit-start", got)
}

func TestResolveStartRevision_FullForcesEmpty(t *testing.T) {
	// --full means "deploy entire branch from first commit" — the API treats
	// an empty start_revision as that signal.
	srv := &sdk.Server{Identifier: "srv-1", LastRevision: "last-deploy-sha"}
	got := resolveStartRevision(srv, "", true)
	assert.Equal(t, "", got, "--full must clear start_revision even when server has a last_revision")
}

func TestResolveStartRevision_FullBeatsExplicitFlag(t *testing.T) {
	// Defensive: even if both somehow get through (the cobra-level mutual
	// exclusivity check should catch it), --full wins.
	srv := &sdk.Server{Identifier: "srv-1", LastRevision: "last-deploy-sha"}
	got := resolveStartRevision(srv, "explicit-start", true)
	assert.Equal(t, "", got)
}

func TestResolveStartRevision_NilServerReturnsEmpty(t *testing.T) {
	// Server-group deploys and project-wide deploys have no single Server to
	// read from — fall back to "" and let the API decide per server.
	got := resolveStartRevision(nil, "", false)
	assert.Equal(t, "", got)
}

func TestResolveStartRevision_NilServerStillRespectsFlag(t *testing.T) {
	// Even without a resolved server, an explicit --start-revision must be honored.
	got := resolveStartRevision(nil, "explicit-start", false)
	assert.Equal(t, "explicit-start", got)
}

func TestResolveStartRevision_FreshServerReturnsEmpty(t *testing.T) {
	// A server that has never been deployed has LastRevision="". First deploy
	// must be a full one — there's no incremental baseline to start from.
	srv := &sdk.Server{Identifier: "srv-1", LastRevision: ""}
	got := resolveStartRevision(srv, "", false)
	assert.Equal(t, "", got)
}

func TestDeployFullMutuallyExclusiveWithStartRevision(t *testing.T) {
	cmd := NewRootCmd("test")
	cmd.SetArgs([]string{"deploy", "--full", "--start-revision", "abc123", "-p", "test-project"})
	err := cmd.Execute()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "mutually exclusive")
}

func TestDeployStartRevisionFlagsRegistered(t *testing.T) {
	cmd := NewRootCmd("test")
	deployCmd, _, _ := cmd.Find([]string{"deploy"})
	require.NotNil(t, deployCmd)
	assert.NotNil(t, deployCmd.Flags().Lookup("start-revision"))
	assert.NotNil(t, deployCmd.Flags().Lookup("full"))
}

func TestResolveGroupName_ExactMatch(t *testing.T) {
	// DHQ-586: passing the group's display name to -s must resolve to its identifier.
	groups := []sdk.ServerGroup{
		{Identifier: "grp-prod", Name: "Production"},
		{Identifier: "grp-stag", Name: "Staging"},
	}
	id, name := resolveGroupName("Production", groups)
	assert.Equal(t, "grp-prod", id)
	assert.Equal(t, "Production", name)
}

func TestResolveGroupName_CaseInsensitive(t *testing.T) {
	groups := []sdk.ServerGroup{{Identifier: "grp-prod", Name: "Production"}}
	id, _ := resolveGroupName("production", groups)
	assert.Equal(t, "grp-prod", id)
}

func TestResolveGroupName_NormalizedMatch(t *testing.T) {
	// "us-prod" should match "US Prod" via the normalize tier.
	groups := []sdk.ServerGroup{
		{Identifier: "grp-us", Name: "US Prod"},
		{Identifier: "grp-eu", Name: "EU Prod"},
	}
	id, name := resolveGroupName("us-prod", groups)
	assert.Equal(t, "grp-us", id)
	assert.Equal(t, "US Prod", name)
}

func TestResolveGroupName_ContainsMatch(t *testing.T) {
	groups := []sdk.ServerGroup{
		{Identifier: "grp-prod", Name: "Production Cluster"},
		{Identifier: "grp-stag", Name: "Staging Cluster"},
	}
	id, _ := resolveGroupName("Production", groups)
	assert.Equal(t, "grp-prod", id)
}

func TestResolveGroupName_AmbiguousReturnsEmpty(t *testing.T) {
	// Multiple contains matches → don't auto-pick. Caller falls back to the
	// existing server picker / "specify which one" error.
	groups := []sdk.ServerGroup{
		{Identifier: "grp-1", Name: "Production US"},
		{Identifier: "grp-2", Name: "Production EU"},
	}
	id, _ := resolveGroupName("Production", groups)
	assert.Equal(t, "", id)
}

func TestResolveGroupName_NoMatch(t *testing.T) {
	groups := []sdk.ServerGroup{{Identifier: "grp-prod", Name: "Production"}}
	id, _ := resolveGroupName("Nonexistent", groups)
	assert.Equal(t, "", id)
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
