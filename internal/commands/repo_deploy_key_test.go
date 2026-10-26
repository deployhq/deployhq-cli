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

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/deployhq/deployhq-cli/internal/output"
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
		case r.Method == http.MethodGet && r.URL.Path == "/projects":
			// No project with this name yet (launch may check before creating).
			_, _ = w.Write([]byte(`[]`))
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
	assert.Contains(t, le.NextStep, "--project my-app")
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
	assert.Contains(t, le.NextStep, "--project my-app")
	assert.NotContains(t, le.NextStep, "github.com/", "no GitHub URL for a non-GitHub host")
}

// Any other repository failure after launch created the project must also say
// to reuse that project; a plain retry would create a second, suffixed one.
func TestLaunchCreateProject_OtherRepoFailures_SayToReuseProject(t *testing.T) {
	noGH(t)
	for name, handler := range otherRepoFailures() {
		for _, remote := range []string{"git@github.com:acme/my-app.git", "https://github.com/acme/my-app.git"} {
			t.Run(name+" "+remote, func(t *testing.T) {
				srv := repoServer(t, handler)
				env, _, _ := testLaunchEnvelope()
				cfg := launchConfig{targetProtocol: "static_hosting", branch: "main"}
				_, err := launchCreateProject(t.Context(), env, cfg, newTestClient(t, srv), remote)
				require.Error(t, err)

				var le *launchError
				require.True(t, isLaunchErr(err, &le), "must be a launchError, got %T: %v", err, err)
				assert.Equal(t, reasonRepoUnreachable, le.Reason)
				assert.Contains(t, le.NextStep, "--project my-app")
				assert.Equal(t, "my-app", le.Details["project"])
				assert.NotContains(t, le.NextStep, testDeployPubKey)
			})
		}
	}
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

// Only DeployHQ's "couldn't connect with the authentication method" refusal
// means the deploy key is missing. Any other failure (a server error, a
// different validation error) passes through unchanged, keeping its exit code
// and retryability.
func otherRepoFailures() map[string]http.HandlerFunc {
	return map[string]http.HandlerFunc{
		"server error": func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusServiceUnavailable)
		},
		"other validation error": func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusUnprocessableEntity)
			json.NewEncoder(w).Encode(map[string][]string{"errors": {"url is invalid"}}) //nolint:errcheck
		},
	}
}

func TestRepoConnectError_SSH_OtherFailuresUnchanged(t *testing.T) {
	for name, handler := range otherRepoFailures() {
		t.Run(name, func(t *testing.T) {
			client := newTestClient(t, repoServer(t, handler))
			_, createErr := client.CreateRepository(t.Context(), "my-app", repoCreateReq("git@github.com:acme/my-app.git"))
			require.Error(t, createErr)

			err := repoConnectError(t.Context(), client, "my-app", "git", "git@github.com:acme/my-app.git", "main", createErr)
			assert.Equal(t, createErr, err)
		})
	}
}

func TestLaunchRepoKeyError_OtherFailuresGetNoKeyInstructions(t *testing.T) {
	for name, handler := range otherRepoFailures() {
		t.Run(name, func(t *testing.T) {
			client := newTestClient(t, repoServer(t, handler))
			_, createErr := client.CreateRepository(t.Context(), "my-app", repoCreateReq("git@github.com:acme/my-app.git"))
			require.Error(t, createErr)

			err := launchRepoKeyError(t.Context(), client, "my-app", nil, "git@github.com:acme/my-app.git", createErr)
			assert.NotContains(t, err.Error(), testDeployPubKey)
			var le *launchError
			if isLaunchErr(err, &le) {
				assert.NotContains(t, le.NextStep, testDeployPubKey)
				assert.Empty(t, le.Details["public_key"])
			}
		})
	}
}

func TestGitHubDeployKeySettingsURL(t *testing.T) {
	assert.Equal(t, "https://github.com/acme/app/settings/keys/new", githubDeployKeySettingsURL("git@github.com:acme/app.git"))
	assert.Equal(t, "https://github.com/acme/app/settings/keys/new", githubDeployKeySettingsURL("ssh://git@github.com/acme/app.git"))
	assert.Equal(t, "", githubDeployKeySettingsURL("git@gitlab.com:acme/app.git"))
}

func TestIsSSHRepoURL(t *testing.T) {
	tests := []struct {
		url  string
		want bool
	}{
		{"git@github.com:acme/app.git", true},
		{"deploy@git.example.com:org/repo.git", true},
		{"hg@bitbucket.org:acme/app", true},
		{"ssh://git@host/acme/app.git", true},
		{"ssh://host:2222/acme/app.git", true},
		{"https://github.com/acme/app.git", false},
		{"http://user@git.example.com/org/repo.git", false},
		{"https://user@git.example.com:8443/org/repo.git", false},
		{"/srv/git/app.git", false},
		{"./app.git", false},
		{"../repos/user@host:app", false},
		{"", false},
	}
	for _, tt := range tests {
		t.Run(tt.url, func(t *testing.T) {
			assert.Equal(t, tt.want, isSSHRepoURL(tt.url))
		})
	}
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
	m.client = newTestClient(t, repoServer(t, rejectRepo))
	m.Update(m.createRepo())
	assert.Equal(t, stepDeployKeyManual, m.step)

	m.Update(createResultMsg{repoOK: true, step: stepRepoURL})
	assert.Equal(t, stepProtocol, m.step)
	assert.True(t, m.repoOK)
}

// `dhq init`: only DeployHQ refusing access to the repository points at the
// deploy key. Any other failure connecting the repository (a timeout, a server
// error, another validation error) goes back to the branch step with the
// original error, where Enter retries and backspace returns to the URL.
func TestInitWizard_SSHRepo_OtherFailuresDoNotBlameDeployKey(t *testing.T) {
	for name, handler := range otherRepoFailures() {
		t.Run(name, func(t *testing.T) {
			m := &initModel{
				step:    stepDeployKeyManual,
				repoURL: "git@github.com:acme/my-app.git",
				scmType: "git",
				branch:  "main",
				client:  newTestClient(t, repoServer(t, handler)),
				project: &sdk.Project{Name: "my-app", Permalink: "my-app", PublicKey: testDeployPubKey},
			}
			msg := m.createRepo()
			resultErr := msg.(createResultMsg).err
			require.Error(t, resultErr)

			m.Update(msg)
			assert.Equal(t, stepBranch, m.step)
			assert.Equal(t, resultErr.Error(), m.err)
			assert.Equal(t, "main", m.input, "Enter on the branch step retries with the same branch")
			assert.NotContains(t, m.View(), testDeployPubKey)
		})
	}
}

// Pressing Enter again on the deploy-key step while the repository is being
// connected must not send a second connection request.
func TestInitWizard_DeployKeyManual_EnterWhileConnectingIsIgnored(t *testing.T) {
	m := &initModel{
		step:    stepDeployKeyManual,
		repoURL: "git@github.com:acme/my-app.git",
		scmType: "git",
		project: &sdk.Project{Name: "my-app", Permalink: "my-app", PublicKey: testDeployPubKey},
	}
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	require.NotNil(t, cmd, "the first Enter connects the repository")
	assert.True(t, m.creating)

	_, cmd = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	assert.Nil(t, cmd, "a second Enter while connecting must not connect again")
	assert.True(t, m.creating)
}

// Going back while the connection is in flight would let its late result move
// the wizard on from the wrong step, so Back waits until it finishes.
func TestInitWizard_DeployKeyManual_BackWhileConnectingIsIgnored(t *testing.T) {
	m := &initModel{
		step:    stepDeployKeyManual,
		repoURL: "git@github.com:acme/my-app.git",
		scmType: "git",
		project: &sdk.Project{Name: "my-app", Permalink: "my-app", PublicKey: testDeployPubKey},
	}
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	require.NotNil(t, cmd)

	m.Update(tea.KeyMsg{Type: tea.KeyBackspace})
	assert.Equal(t, stepDeployKeyManual, m.step, "Back is ignored while connecting")

	m.creating = false
	m.Update(tea.KeyMsg{Type: tea.KeyBackspace})
	assert.Equal(t, stepBranch, m.step, "Back works again once the connection finished")
}

// Once gh has added the key, the repository is connected while the wizard is
// still on the automatic deploy-key step. Enter must not run gh again or start
// a second connection, nor switch to the manual step.
func TestInitWizard_DeployKeyAuto_EnterWhileConnectingIsIgnored(t *testing.T) {
	for _, cursor := range []int{0, 1} {
		m := &initModel{
			step:        stepDeployKeyAuto,
			repoURL:     "git@github.com:acme/my-app.git",
			scmType:     "git",
			ghAvailable: true,
			project:     &sdk.Project{Name: "my-app", Permalink: "my-app", PublicKey: testDeployPubKey},
		}
		_, cmd := m.Update(createResultMsg{step: stepDeployKeyAuto})
		require.NotNil(t, cmd, "the key was added, so the repository is being connected")
		require.True(t, m.creating)

		m.cursor = cursor
		_, cmd = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
		assert.Nil(t, cmd, "Enter while connecting must not run gh or connect again (cursor %d)", cursor)
		assert.Equal(t, stepDeployKeyAuto, m.step, "Enter while connecting must not leave the step (cursor %d)", cursor)
		assert.True(t, m.creating)
	}
}

// Going back while the connection is in flight would let its late result move
// the wizard on from the wrong step, so Back waits until it finishes.
func TestInitWizard_DeployKeyAuto_BackWhileConnectingIsIgnored(t *testing.T) {
	m := &initModel{
		step:        stepDeployKeyAuto,
		repoURL:     "git@github.com:acme/my-app.git",
		scmType:     "git",
		ghAvailable: true,
		project:     &sdk.Project{Name: "my-app", Permalink: "my-app", PublicKey: testDeployPubKey},
	}
	_, cmd := m.Update(createResultMsg{step: stepDeployKeyAuto})
	require.NotNil(t, cmd)

	m.Update(tea.KeyMsg{Type: tea.KeyBackspace})
	assert.Equal(t, stepDeployKeyAuto, m.step, "Back is ignored while connecting")

	m.creating = false
	m.Update(tea.KeyMsg{Type: tea.KeyBackspace})
	assert.Equal(t, stepBranch, m.step, "Back works again once the connection finished")
}

// gh added the key but connecting the repository then failed for another
// reason (say a 5xx), so the wizard went back to the branch step. Submitting
// the branch again returns to the automatic step, where Enter must retry the
// repository request rather than run gh again, which would fail on the
// duplicate key and send the person to the manual step.
func TestInitWizard_DeployKeyAuto_KeyAlreadyAdded_EnterRetriesRepository(t *testing.T) {
	for _, cursor := range []int{0, 1} {
		m := &initModel{
			step:        stepDeployKeyAuto,
			repoURL:     "git@github.com:acme/my-app.git",
			scmType:     "git",
			branch:      "main",
			ghAvailable: true,
			project:     &sdk.Project{Name: "my-app", Permalink: "my-app", PublicKey: testDeployPubKey},
		}
		_, cmd := m.Update(createResultMsg{step: stepDeployKeyAuto})
		require.NotNil(t, cmd)
		require.True(t, m.deployKeyAdded)

		m.Update(createResultMsg{err: assert.AnError, step: stepRepoURL})
		require.Equal(t, stepBranch, m.step)
		require.False(t, m.creating)

		_, cmd = m.submitTextStep()
		require.Nil(t, cmd)
		require.Equal(t, stepDeployKeyAuto, m.step)
		assert.Contains(t, m.View(), "retry connecting the repository")

		ghLog := fakeGH(t, 1)
		creates := 0
		m.client = newTestClient(t, repoServer(t, func(w http.ResponseWriter, _ *http.Request) {
			creates++
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{}`))
		}))

		m.cursor = cursor
		_, cmd = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
		require.NotNil(t, cmd, "Enter retries the repository (cursor %d)", cursor)
		assert.True(t, m.creating)
		assert.Equal(t, stepDeployKeyAuto, m.step)

		msg := cmd()
		res, ok := msg.(createResultMsg)
		require.True(t, ok)
		assert.Equal(t, stepRepoURL, res.step, "Enter must connect the repository, not run gh (cursor %d)", cursor)
		assert.Equal(t, 1, creates, "the repository request is sent once (cursor %d)", cursor)
		assert.False(t, ghWasCalled(ghLog), "gh must not run again (cursor %d)", cursor)

		m.Update(msg)
		assert.Equal(t, stepProtocol, m.step)
	}
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

// In --json mode the rejected-repository error carries the key in next_step.
func TestLaunchRepoKeyError_JSONShape(t *testing.T) {
	srv := repoServer(t, rejectRepo)
	client := newTestClient(t, srv)
	_, createErr := client.CreateRepository(t.Context(), "my-app", repoCreateReq("git@github.com:acme/my-app.git"))
	require.Error(t, createErr)

	keyErr := launchRepoKeyError(t.Context(), client, "my-app", nil, "git@github.com:acme/my-app.git", createErr)
	env, stdout, _ := testLaunchEnvelopeJSON()
	_ = writeLaunchError(env, launchConfig{}, reasonRepoUnreachable, keyErr)

	var resp struct {
		OK   bool `json:"ok"`
		Data struct {
			Reason   string            `json:"reason"`
			NextStep string            `json:"next_step"`
			Details  map[string]string `json:"details"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(stdout.Bytes(), &resp))
	assert.False(t, resp.OK)
	assert.Equal(t, reasonRepoUnreachable, resp.Data.Reason)
	assert.Contains(t, resp.Data.NextStep, testDeployPubKey)
	assert.Contains(t, resp.Data.NextStep, "https://github.com/acme/my-app/settings/keys/new")
	assert.Equal(t, testDeployPubKey, resp.Data.Details["public_key"])
}

// Over SSH a public repository still needs the key, so the prompt must not
// suggest skipping it; over HTTPS a public repository can.
func TestDeployKeyPausePrompt(t *testing.T) {
	ssh := deployKeyPausePrompt("git@github.com:acme/my-app.git")
	assert.NotContains(t, ssh, "or now")
	assert.Contains(t, ssh, "even for a public repository")

	assert.Contains(t, deployKeyPausePrompt("https://github.com/acme/my-app.git"), "or now, if the repo is public")
}

// In --json mode writeLaunchError writes the only JSON object on stdout and
// marks the error as reported, so the top-level handler adds nothing.
func TestWriteLaunchError_JSON_SingleObject(t *testing.T) {
	env, stdout, _ := testLaunchEnvelopeJSON()
	err := writeLaunchError(env, launchConfig{}, reasonRepoUnreachable, &launchError{Reason: reasonRepoUnreachable, Message: "rejected"})
	assert.True(t, output.IsReported(err))

	dec := json.NewDecoder(stdout)
	var first map[string]interface{}
	require.NoError(t, dec.Decode(&first))
	assert.False(t, dec.More(), "stdout must hold exactly one JSON object")

	// That one object must still carry what the standard error object did.
	data := first["data"].(map[string]interface{})
	assert.Equal(t, float64(output.ClassifyError(err)), data["exit_code"], "exit_code must match the process exit code")
	assert.NotZero(t, data["exit_code"])
	assert.Contains(t, data, "recovery")

	// Plain mode is untouched: nothing reported, main prints the error.
	plainEnv, _, _ := testLaunchEnvelope()
	plainEnv.IsTTY = true // a non-TTY envelope auto-selects JSON
	assert.False(t, output.IsReported(writeLaunchError(plainEnv, launchConfig{}, reasonRepoUnreachable, &launchError{Message: "x"})))
}

func TestQuoteRerunArg(t *testing.T) {
	cases := map[string]string{
		"main":                           "main",
		"feature/x-1.2_@v%+=:,":          "feature/x-1.2_@v%+=:,",
		"git@github.com:acme/my-app.git": "git@github.com:acme/my-app.git",
		"release&hotfix":                 "'release&hotfix'",
		"it's":                           `'it'\''s'`,
		"a b":                            "'a b'",
		"$(id)":                          "'$(id)'",
		"":                               "''",
	}
	for in, want := range cases {
		assert.Equal(t, want, quoteRerunArg(in), "input %q", in)
	}
}

// Values in the suggested re-run command are shell-quoted when needed, so a
// valid branch like release&hotfix does not run shell syntax when pasted.
func TestRepoConnectError_SSH_QuotesRerunValues(t *testing.T) {
	cases := map[string]string{
		"release&hotfix": "--branch 'release&hotfix'",
		"it's":           `--branch 'it'\''s'`,
		"main":           "--branch main",
	}
	for branch, want := range cases {
		t.Run(branch, func(t *testing.T) {
			client := newTestClient(t, repoServer(t, rejectRepo))
			_, createErr := client.CreateRepository(t.Context(), "my-app", repoCreateReq("git@github.com:acme/my-app.git"))
			require.Error(t, createErr)

			err := repoConnectError(t.Context(), client, "my-app", "git", "git@github.com:acme/my-app.git", branch, createErr)
			hint := err.Error()
			var ue *output.UserError
			if assert.ErrorAs(t, err, &ue) {
				hint = ue.Hint
			}
			assert.Contains(t, hint, "dhq repos create -p my-app --scm-type git --url git@github.com:acme/my-app.git "+want)
		})
	}
}

// The gh command in the deploy-key instructions quotes the repository taken
// from the URL the person typed.
func TestDeployKeyInstructions_QuotesGHRepo(t *testing.T) {
	got := deployKeyInstructions("git@github.com:acme/my&app.git", testDeployPubKey, "")
	assert.Contains(t, got, "-R 'acme/my&app' --title DeployHQ")
	plain := deployKeyInstructions("git@github.com:acme/my-app.git", testDeployPubKey, "")
	assert.Contains(t, plain, "-R acme/my-app --title DeployHQ")
}

// The --project value in launch's resume instruction is quoted too.
func TestLaunchRepoErrors_QuoteProject(t *testing.T) {
	client := newTestClient(t, repoServer(t, rejectRepo))
	_, createErr := client.CreateRepository(t.Context(), "my-app", repoCreateReq("git@github.com:acme/my-app.git"))
	require.Error(t, createErr)

	proj := &sdk.Project{PublicKey: testDeployPubKey}
	var le *launchError
	require.True(t, isLaunchErr(launchRepoKeyError(t.Context(), client, "a&b", proj, "git@github.com:acme/my-app.git", createErr), &le))
	assert.Contains(t, le.NextStep, "--project 'a&b' added")

	require.True(t, isLaunchErr(launchNewProjectRepoFailure(assert.AnError, "a&b"), &le))
	assert.Contains(t, le.NextStep, "--project 'a&b' added")
}

// --json=<fields> must not filter launch's error away: it is the only JSON
// object written for the failure, so reason, exit_code and recovery have to
// survive a field selection aimed at the success payload.
func TestWriteLaunchError_JSONFieldsDoNotFilterError(t *testing.T) {
	env, stdout, _ := testLaunchEnvelopeJSON()
	env.JSONFields = []string{"url"}
	err := writeLaunchError(env, launchConfig{}, reasonRepoUnreachable, &launchError{Reason: reasonRepoUnreachable, Message: "rejected"})
	assert.True(t, output.IsReported(err))

	var got map[string]interface{}
	require.NoError(t, json.Unmarshal(stdout.Bytes(), &got))
	data, ok := got["data"].(map[string]interface{})
	require.True(t, ok, "error payload missing: %s", stdout.String())
	assert.Equal(t, reasonRepoUnreachable, data["reason"])
	assert.NotZero(t, data["exit_code"])
	assert.Contains(t, data, "recovery")
}

// The public key's comment carries the project name, so a ' in it must not
// break the copy-paste gh command: every place that echoes the key quotes it.
func TestDeployKeyCommands_QuotePublicKey(t *testing.T) {
	const key = "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAItestkey DeployHQ.com Key for it's"
	const want = `echo 'ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAItestkey DeployHQ.com Key for it'\''s' | gh repo deploy-key add -R acme/my-app --title DeployHQ -`
	const repoURL = "git@github.com:acme/my-app.git"

	assert.Contains(t, deployKeyInstructions(repoURL, key, ""), want)

	crumbs := deployKeyBreadcrumbs(repoURL, key, "my-app")
	require.NotEmpty(t, crumbs)
	assert.Equal(t, want, crumbs[0].Cmd)

	env, _, stderr := testLaunchEnvelope()
	printDeployKeyHint(env, repoURL, key, "my-app")
	assert.Contains(t, stderr.String(), want)
}

// A deploy key added for one repository says nothing about another: entering a
// different repository URL must offer to add the key again.
func TestInitWizard_ChangedRepoURL_ForgetsAddedDeployKey(t *testing.T) {
	m := &initModel{
		step:           stepRepoURL,
		repoURL:        "git@github.com:acme/my-app.git",
		deployKeyAdded: true,
		project:        &sdk.Project{Name: "my-app", Permalink: "my-app", PublicKey: testDeployPubKey},
	}
	m.input = "git@github.com:acme/other-app.git"
	m.submitTextStep()
	assert.False(t, m.deployKeyAdded, "a new repository URL needs its own deploy key")

	same := &initModel{
		step:           stepRepoURL,
		repoURL:        "git@github.com:acme/my-app.git",
		deployKeyAdded: true,
		project:        &sdk.Project{Name: "my-app", Permalink: "my-app", PublicKey: testDeployPubKey},
	}
	same.input = "git@github.com:acme/my-app.git"
	same.submitTextStep()
	assert.True(t, same.deployKeyAdded, "re-entering the same URL keeps the added key")
}
