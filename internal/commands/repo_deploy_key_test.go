package commands

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/deployhq/deployhq-cli/pkg/sdk"
)

const testDeployPubKey = "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAItestkey DeployHQ.com Key for my-app"

// repoRejectedBody is the shape the API returns when it cannot read the
// repository with the project's deploy key.
var repoRejectedBody = map[string][]string{"errors": {
	"base We couldn't connect to that repository using the authentication method provided. Please ensure you have uploaded the appropriate public key OR entered the correct credentials.",
}}

// fakeGH puts a stub `gh` first on PATH. Each invocation appends its arguments
// to the returned log file and exits with exitCode. PATH holds only the stub's
// directory, so no real gh can be reached.
func fakeGH(t *testing.T, exitCode int) string {
	t.Helper()
	dir := t.TempDir()
	logFile := filepath.Join(dir, "gh.log")
	script := "#!/bin/sh\necho \"$@\" >> \"" + logFile + "\"\nexit " + strconv.Itoa(exitCode) + "\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "gh"), []byte(script), 0o755)) //nolint:gosec // test stub must be executable
	t.Setenv("PATH", dir)
	return logFile
}

// noGH empties PATH so `gh` cannot be found.
func noGH(t *testing.T) {
	t.Helper()
	t.Setenv("PATH", t.TempDir())
}

func ghWasCalled(logFile string) bool {
	_, err := os.Stat(logFile)
	return err == nil
}

// repoServer serves a project with no repository and the test deploy key.
// onCreate handles POST /projects/my-app/repository.
func repoServer(t *testing.T, onCreate http.HandlerFunc) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/projects":
			json.NewEncoder(w).Encode(map[string]interface{}{ //nolint:errcheck
				"name": "my-app", "permalink": "my-app", "public_key": testDeployPubKey,
			})
		case r.Method == http.MethodGet && r.URL.Path == "/projects/my-app":
			json.NewEncoder(w).Encode(map[string]interface{}{ //nolint:errcheck
				"name": "my-app", "permalink": "my-app", "public_key": testDeployPubKey,
			})
		case r.Method == http.MethodPost && r.URL.Path == "/projects/my-app/repository":
			onCreate(w, r)
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusInternalServerError)
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

func rejectRepo(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusUnprocessableEntity)
	json.NewEncoder(w).Encode(repoRejectedBody) //nolint:errcheck
}

func acceptRepo(w http.ResponseWriter, _ *http.Request) {
	json.NewEncoder(w).Encode(map[string]interface{}{ //nolint:errcheck
		"scm_type": "git", "url": "git@github.com:acme/my-app.git", "branch": "main",
	})
}

// For a GitHub SSH URL the deploy key must be on GitHub BEFORE the repository
// is connected: DeployHQ checks access with that key on connect.
func TestLaunchEnsureRepo_GitHubSSH_AddsDeployKeyBeforeConnecting(t *testing.T) {
	ghLog := fakeGH(t, 0)
	keyAddedFirst := false
	srv := repoServer(t, func(w http.ResponseWriter, r *http.Request) {
		keyAddedFirst = ghWasCalled(ghLog)
		acceptRepo(w, r)
	})

	env, _, _ := testLaunchEnvelope()
	cfg := launchConfig{targetProtocol: "static_hosting", branch: "main"}
	err := launchEnsureRepo(t.Context(), env, cfg, newTestClient(t, srv), "my-app", "git@github.com:acme/my-app.git")
	require.NoError(t, err)
	assert.True(t, keyAddedFirst, "gh deploy-key add must run before the repository is connected")

	args, readErr := os.ReadFile(ghLog)
	require.NoError(t, readErr)
	assert.Contains(t, string(args), "repo deploy-key add")
	assert.Contains(t, string(args), "--repo acme/my-app")
}

// If gh fails (for example the key is already on the repository from an
// earlier run), connecting is still attempted and can succeed.
func TestLaunchEnsureRepo_GHFails_StillConnects(t *testing.T) {
	fakeGH(t, 1)
	created := false
	srv := repoServer(t, func(w http.ResponseWriter, r *http.Request) {
		created = true
		acceptRepo(w, r)
	})

	env, _, _ := testLaunchEnvelope()
	cfg := launchConfig{targetProtocol: "static_hosting", branch: "main"}
	err := launchEnsureRepo(t.Context(), env, cfg, newTestClient(t, srv), "my-app", "git@github.com:acme/my-app.git")
	require.NoError(t, err)
	assert.True(t, created)
}

// When the key cannot be added automatically and the repository is rejected,
// launch must stop with the key, the GitHub settings URL and the re-run
// command — not carry on with a project that has no repository.
func TestLaunchEnsureRepo_GitHubSSH_NoGH_StopsWithInstructions(t *testing.T) {
	noGH(t)
	srv := repoServer(t, rejectRepo)

	env, _, _ := testLaunchEnvelope()
	cfg := launchConfig{targetProtocol: "static_hosting", branch: "main"}
	err := launchEnsureRepo(t.Context(), env, cfg, newTestClient(t, srv), "my-app", "git@github.com:acme/my-app.git")
	require.Error(t, err)

	var le *launchError
	require.True(t, isLaunchErr(err, &le), "must be a launchError, got %T: %v", err, err)
	assert.Equal(t, reasonRepoUnreachable, le.Reason)
	assert.Contains(t, le.NextStep, testDeployPubKey)
	assert.Contains(t, le.NextStep, "https://github.com/acme/my-app/settings/keys/new")
	assert.Contains(t, le.NextStep, "dhq launch --project my-app")
}

// A newly created project whose repository is rejected must stop launch
// instead of warning and continuing to provisioning.
func TestLaunchCreateProject_RepoRejected_Stops(t *testing.T) {
	noGH(t)
	srv := repoServer(t, rejectRepo)

	env, _, _ := testLaunchEnvelope()
	cfg := launchConfig{targetProtocol: "static_hosting", branch: "main"}
	permalink, err := launchCreateProject(t.Context(), env, cfg, newTestClient(t, srv), "git@git.example.com:acme/my-app.git")
	require.Error(t, err, "a project without its repository must not be handed to later steps")
	assert.Empty(t, permalink)

	var le *launchError
	require.True(t, isLaunchErr(err, &le))
	assert.Equal(t, reasonRepoUnreachable, le.Reason)
	assert.Contains(t, le.NextStep, testDeployPubKey)
	assert.Contains(t, le.NextStep, "dhq launch --project my-app")
	assert.NotContains(t, le.NextStep, "github.com/", "no GitHub URL for a non-GitHub host")
}

// An existing project whose repository is rejected keeps a single, clear
// error (no double "Could not connect" wrapping).
func TestLaunchEnsureProject_ExistingProject_RepoRejected_NoDoubleWrap(t *testing.T) {
	noGH(t)
	srv := repoServer(t, rejectRepo)

	env, _, _ := testLaunchEnvelope()
	cfg := launchConfig{targetProtocol: "static_hosting", branch: "main", projectID: "my-app"}
	_, err := launchEnsureProject(t.Context(), env, cfg, newTestClient(t, srv), "git@github.com:acme/my-app.git")
	require.Error(t, err)

	var le *launchError
	require.True(t, isLaunchErr(err, &le))
	assert.Equal(t, 1, strings.Count(le.Message, "Could not connect repository"))
	assert.Contains(t, le.NextStep, "https://github.com/acme/my-app/settings/keys/new")
}

// HTTPS URLs are not access-checked on connect, so no key is added first.
func TestLaunchEnsureRepo_HTTPS_NoKeyBeforeConnect(t *testing.T) {
	ghLog := fakeGH(t, 0)
	keyAddedFirst := false
	srv := repoServer(t, func(w http.ResponseWriter, r *http.Request) {
		keyAddedFirst = ghWasCalled(ghLog)
		acceptRepo(w, r)
	})

	env, _, _ := testLaunchEnvelope()
	cfg := launchConfig{targetProtocol: "static_hosting", branch: "main"}
	err := launchEnsureRepo(t.Context(), env, cfg, newTestClient(t, srv), "my-app", "https://github.com/acme/my-app.git")
	require.NoError(t, err)
	assert.False(t, keyAddedFirst, "an HTTPS repository is connected without adding a key first")
}

// `dhq repos create` on a rejected SSH repository must explain the deploy key
// instead of returning only the server's generic message.
func TestRepoConnectError_SSH_IncludesDeployKeyInstructions(t *testing.T) {
	srv := repoServer(t, rejectRepo)
	client := newTestClient(t, srv)

	_, createErr := client.CreateRepository(t.Context(), "my-app", repoCreateReq("git@github.com:acme/my-app.git"))
	require.Error(t, createErr)

	err := repoConnectError(t.Context(), client, "my-app", "git", "git@github.com:acme/my-app.git", "main", createErr)
	msg := err.Error()
	assert.Contains(t, msg, "couldn't connect to that repository", "server message is kept")
	assert.Contains(t, msg, testDeployPubKey)
	assert.Contains(t, msg, "https://github.com/acme/my-app/settings/keys/new")
	assert.Contains(t, msg, "dhq repos create -p my-app --scm-type git --url git@github.com:acme/my-app.git --branch main")
}

func TestRepoConnectError_HTTPS_Unchanged(t *testing.T) {
	srv := repoServer(t, rejectRepo)
	client := newTestClient(t, srv)

	_, createErr := client.CreateRepository(t.Context(), "my-app", repoCreateReq("https://github.com/acme/my-app.git"))
	require.Error(t, createErr)

	err := repoConnectError(t.Context(), client, "my-app", "git", "https://github.com/acme/my-app.git", "", createErr)
	assert.Equal(t, createErr, err)
}

func TestGitHubDeployKeySettingsURL(t *testing.T) {
	assert.Equal(t, "https://github.com/acme/app/settings/keys/new", githubDeployKeySettingsURL("git@github.com:acme/app.git"))
	assert.Equal(t, "https://github.com/acme/app/settings/keys/new", githubDeployKeySettingsURL("ssh://git@github.com/acme/app.git"))
	assert.Equal(t, "", githubDeployKeySettingsURL("git@gitlab.com:acme/app.git"))
}

func TestIsSSHRepoURL(t *testing.T) {
	assert.True(t, isSSHRepoURL("git@github.com:acme/app.git"))
	assert.True(t, isSSHRepoURL("ssh://git@host/acme/app.git"))
	assert.False(t, isSSHRepoURL("https://github.com/acme/app.git"))
}

func repoCreateReq(url string) sdk.RepositoryCreateRequest {
	return sdk.RepositoryCreateRequest{ScmType: "git", URL: url}
}

// `dhq init`: for an SSH repository the deploy-key step comes BEFORE the
// repository is connected, and a rejected connection stays on that step.
func TestInitWizard_SSHRepo_DeployKeyBeforeConnect(t *testing.T) {
	m := &initModel{
		step:    stepBranch,
		repoURL: "git@github.com:acme/my-app.git",
		scmType: "git",
		project: &sdk.Project{Name: "my-app", Permalink: "my-app", PublicKey: testDeployPubKey},
	}

	_, cmd := m.submitTextStep()
	assert.Nil(t, cmd, "the repository must not be connected before the key step")
	assert.Equal(t, stepDeployKeyManual, m.step)
	assert.Contains(t, m.View(), testDeployPubKey)
	assert.Contains(t, m.View(), "https://github.com/acme/my-app/settings/keys/new")

	// A rejected connection keeps the user on the key step to retry.
	m.Update(createResultMsg{err: assert.AnError, step: stepRepoURL})
	assert.Equal(t, stepDeployKeyManual, m.step)

	m.Update(createResultMsg{repoOK: true, step: stepRepoURL})
	assert.Equal(t, stepProtocol, m.step)
	assert.True(t, m.repoOK)
}

func TestInitWizard_GHKeyAdded_ThenConnects(t *testing.T) {
	m := &initModel{
		step:        stepBranch,
		repoURL:     "git@github.com:acme/my-app.git",
		scmType:     "git",
		ghAvailable: true,
		project:     &sdk.Project{Name: "my-app", Permalink: "my-app", PublicKey: testDeployPubKey},
	}
	_, cmd := m.submitTextStep()
	assert.Nil(t, cmd)
	assert.Equal(t, stepDeployKeyAuto, m.step)

	_, cmd = m.Update(createResultMsg{step: stepDeployKeyAuto})
	assert.NotNil(t, cmd, "adding the key must be followed by connecting the repository")
	assert.True(t, m.deployKeyAdded)

	// gh failing falls back to the manual step.
	m2 := &initModel{step: stepDeployKeyAuto, repoURL: "git@github.com:acme/my-app.git", project: m.project}
	m2.Update(createResultMsg{err: assert.AnError, step: stepDeployKeyAuto})
	assert.Equal(t, stepDeployKeyManual, m2.step)
}

func TestInitWizard_HTTPSRepo_ConnectsDirectly(t *testing.T) {
	m := &initModel{
		step:    stepBranch,
		repoURL: "https://github.com/acme/my-app.git",
		scmType: "git",
		project: &sdk.Project{Name: "my-app", Permalink: "my-app"},
	}
	_, cmd := m.submitTextStep()
	assert.NotNil(t, cmd, "an HTTPS repository is connected straight away")
}
