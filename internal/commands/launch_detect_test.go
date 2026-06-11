package commands

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/deployhq/deployhq-cli/internal/detect"
	"github.com/deployhq/deployhq-cli/pkg/sdk"
	"github.com/stretchr/testify/assert"
)

func TestDetectionResultFromAPI_MapsAllFields(t *testing.T) {
	resp := &sdk.DetectionResponse{
		Stack:             "spa_vite_react",
		SuggestedProtocol: "static_hosting",
		StaticHosting:     sdk.DetectionStaticHosting{RootPath: "dist", SPAMode: true},
		BuildCommands: []sdk.DetectionBuildCommand{
			{Command: "npm install"},
			{Command: "npm run build"},
		},
	}

	r := detectionResultFromAPI(resp)
	assert.Equal(t, detect.Framework("spa_vite_react"), r.Framework)
	assert.Equal(t, "static_hosting", r.SuggestedProtocol)
	assert.Equal(t, "dist", r.OutputDir)
	assert.True(t, r.SPA)
	// Multiple build steps collapse into one shell command.
	assert.Equal(t, "npm install && npm run build", r.BuildCommand)
}

func TestLaunchDetect_UsesRemoteWhenAvailable(t *testing.T) {
	// Spec-validating client: the request the CLI sends must conform to the
	// /detection contract, and the mapped response drives the recommendation.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/detection", r.URL.Path)
		_, _ = w.Write([]byte(`{"stack":"rails","suggested_protocol":"managed_vps",` +
			`"static_hosting":{"eligibility":"requires_runtime","confidence":"none"},"build_commands":[]}`))
	}))
	defer srv.Close()

	env, _, _ := testLaunchEnvelope()
	client := newSpecValidatingClient(t, srv)
	got := launchDetect(t.Context(), env, client, t.TempDir())

	assert.Equal(t, detect.ProtocolManagedVPS, got.SuggestedProtocol)
	assert.Equal(t, detect.Framework("rails"), got.Framework)
}

func TestLaunchDetect_FallsBackToLocalOnRemoteError(t *testing.T) {
	// When the endpoint errors, detection falls back to the local heuristic
	// silently. A local Gemfile must still yield the managed_vps suggestion.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "Gemfile"), []byte("gem 'rails'\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	env, _, _ := testLaunchEnvelope()
	client := newTestClient(t, srv) // plain client: the 500 reaches the fallback path
	got := launchDetect(t.Context(), env, client, dir)

	assert.Equal(t, detect.ProtocolManagedVPS, got.SuggestedProtocol,
		"must fall back to local detection (Gemfile → managed_vps)")
}
