package commands

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os/exec"
	"strings"
	"sync"
	"testing"

	"github.com/deployhq/deployhq-cli/internal/output"
	"github.com/deployhq/deployhq-cli/pkg/sdk"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const launchProjectRemote = "git@github.com:acme/my-app.git"

// launchProjectAPI is a fake API for project resolution. It records every
// project create and repository connect it receives.
type launchProjectAPI struct {
	mu           sync.Mutex
	created      []string
	repoURLs     []string
	createStatus []int
}

func (a *launchProjectAPI) start(t *testing.T, listJSON string) *sdk.Client {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		a.mu.Lock()
		defer a.mu.Unlock()
		raw, _ := io.ReadAll(r.Body)
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/projects":
			_, _ = w.Write([]byte(listJSON))
		case r.Method == http.MethodPost && r.URL.Path == "/projects":
			var body struct {
				Project struct {
					Name string `json:"name"`
				} `json:"project"`
			}
			_ = json.Unmarshal(raw, &body)
			a.created = append(a.created, body.Project.Name)
			if len(a.createStatus) > 0 {
				status := a.createStatus[0]
				a.createStatus = a.createStatus[1:]
				w.WriteHeader(status)
				_, _ = w.Write([]byte(`{"errors":["Project limit reached"]}`))
				return
			}
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"name":"` + body.Project.Name + `","permalink":"new-permalink"}`))
		case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/projects/"):
			_, _ = w.Write([]byte(`{"name":"x","permalink":"` + strings.TrimPrefix(r.URL.Path, "/projects/") + `"}`))
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/repository"):
			var body struct {
				Repository struct {
					URL string `json:"url"`
				} `json:"repository"`
			}
			_ = json.Unmarshal(raw, &body)
			a.repoURLs = append(a.repoURLs, body.Repository.URL)
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{}`))
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusInternalServerError)
		}
	}))
	t.Cleanup(srv.Close)
	return newTestClient(t, srv)
}

// inGitRepoWithRemote moves the test into a new git repository whose origin is
// remote, so a created project is linked there and not in this source tree.
func inGitRepoWithRemote(t *testing.T, remote string) {
	t.Helper()
	dir := t.TempDir()
	t.Chdir(dir)
	for _, args := range [][]string{{"init", "-q"}, {"remote", "add", "origin", remote}} {
		out, err := exec.Command("git", args...).CombinedOutput()
		require.NoError(t, err, string(out))
	}
}

func launchProjectJSON(name, permalink, repoURL string) string {
	repo := ""
	if repoURL != "" {
		repo = `,"repository":{"scm_type":"git","url":"` + repoURL + `"},"repository_url":"` + repoURL + `"}`
	} else {
		repo = "}"
	}
	return `{"name":"` + name + `","permalink":"` + permalink + `"` + repo
}

func launchProjectList(projects ...string) string {
	return "[" + strings.Join(projects, ",") + "]"
}

func TestLaunchEnsureProject_NonInteractive_EmptyAccountCreatesProject(t *testing.T) {
	inGitRepoWithRemote(t, launchProjectRemote)
	api := &launchProjectAPI{}
	client := api.start(t, "[]")
	env, _, _ := testLaunchEnvelope()

	permalink, created, err := launchEnsureProject(t.Context(), env, launchConfig{targetProtocol: "static_hosting", branch: "main"}, client, launchProjectRemote)

	require.NoError(t, err)
	assert.True(t, created, "the caller must know this run created the project")
	assert.Equal(t, "new-permalink", permalink)
	assert.Equal(t, []string{"my-app"}, api.created, "named after the repository")
	assert.Equal(t, []string{launchProjectRemote}, api.repoURLs, "the repository is connected")
}

func TestLaunchEnsureProject_NonInteractive_NoMatchingProjectCreatesOne(t *testing.T) {
	inGitRepoWithRemote(t, launchProjectRemote)
	api := &launchProjectAPI{}
	client := api.start(t, launchProjectList(
		launchProjectJSON("Other", "other", "git@github.com:acme/other.git"),
		launchProjectJSON("Blank", "blank", ""),
	))
	env, _, _ := testLaunchEnvelope()

	permalink, created, err := launchEnsureProject(t.Context(), env, launchConfig{branch: "main"}, client, launchProjectRemote)

	require.NoError(t, err)
	assert.True(t, created)
	assert.Equal(t, "new-permalink", permalink)
	assert.Equal(t, []string{"my-app"}, api.created)
}

func TestLaunchEnsureProject_NonInteractive_ReusesProjectMatchingRemote(t *testing.T) {
	tests := map[string]string{
		"same url":        launchProjectRemote,
		"https form":      "https://github.com/acme/my-app",
		"https with .git": "https://github.com/acme/my-app.git",
		"credentials":     "https://x-token:s3cret@github.com/acme/my-app.git",
		"ssh scheme":      "ssh://git@github.com/acme/my-app.git",
		"different case":  "git@GitHub.com:Acme/My-App.git",
		"trailing slash":  "https://github.com/acme/my-app/",
	}
	for name, projectURL := range tests {
		t.Run(name, func(t *testing.T) {
			inGitRepoWithRemote(t, launchProjectRemote)
			api := &launchProjectAPI{}
			client := api.start(t, launchProjectList(
				launchProjectJSON("Other", "other", "git@github.com:acme/other.git"),
				launchProjectJSON("Renamed Site", "renamed-site", projectURL),
			))
			env, _, _ := testLaunchEnvelope()

			permalink, created, err := launchEnsureProject(t.Context(), env, launchConfig{branch: "main"}, client, launchProjectRemote)

			require.NoError(t, err)
			assert.False(t, created)
			assert.Equal(t, "renamed-site", permalink)
			assert.Empty(t, api.created, "must not create a second project for this repository")
		})
	}
}

func TestLaunchEnsureProject_NonInteractive_TwoMatchingProjectsRefused(t *testing.T) {
	inGitRepoWithRemote(t, launchProjectRemote)
	api := &launchProjectAPI{}
	client := api.start(t, launchProjectList(
		launchProjectJSON("One", "one", launchProjectRemote),
		launchProjectJSON("Two", "two", "https://github.com/acme/my-app.git"),
	))
	env, _, _ := testLaunchEnvelope()

	_, created, err := launchEnsureProject(t.Context(), env, launchConfig{branch: "main"}, client, launchProjectRemote)

	require.Error(t, err)
	assert.False(t, created)
	var le *launchError
	require.True(t, isLaunchErr(err, &le), "got %v", err)
	assert.Equal(t, reasonProjectAmbiguous, le.Reason)
	assert.Equal(t, output.ExitUserError, le.ExitCode())
	assert.Contains(t, le.Message, "one")
	assert.Contains(t, le.Message, "two")
	assert.Contains(t, le.NextStep, "--project")
	assert.Empty(t, api.created)
	assert.Empty(t, api.repoURLs)
}

// A project already using the repository's name, on another repository or none,
// may be the one the person means. Launch has no --allow-duplicate to offer.
func TestLaunchEnsureProject_NonInteractive_SameNamedProjectRefused(t *testing.T) {
	inGitRepoWithRemote(t, launchProjectRemote)
	api := &launchProjectAPI{}
	client := api.start(t, launchProjectList(
		launchProjectJSON("my-app", "my-app-1", ""),
		launchProjectJSON("Other", "other", "git@github.com:acme/other.git"),
	))
	env, _, _ := testLaunchEnvelope()

	_, _, err := launchEnsureProject(t.Context(), env, launchConfig{branch: "main"}, client, launchProjectRemote)

	require.Error(t, err)
	var le *launchError
	require.True(t, isLaunchErr(err, &le), "got %v", err)
	assert.Equal(t, reasonProjectAmbiguous, le.Reason)
	assert.Contains(t, le.NextStep, "--project my-app-1")
	assert.NotContains(t, le.Error(), "--allow-duplicate", "launch has no such flag")
	assert.Empty(t, api.created)
}

func TestLaunchEnsureProject_NonInteractive_SoleProjectStillAutoSelected(t *testing.T) {
	inGitRepoWithRemote(t, launchProjectRemote)
	api := &launchProjectAPI{}
	client := api.start(t, launchProjectList(launchProjectJSON("Only", "only", launchProjectRemote)))
	env, _, _ := testLaunchEnvelope()

	permalink, created, err := launchEnsureProject(t.Context(), env, launchConfig{branch: "main"}, client, launchProjectRemote)

	require.NoError(t, err)
	assert.False(t, created)
	assert.Equal(t, "only", permalink)
	assert.Empty(t, api.created)
}

func TestLaunchEnsureProject_NonInteractive_CreateFailureIsExplained(t *testing.T) {
	tests := map[string]struct {
		statuses []int
		exitCode int
	}{
		"refused by the api": {[]int{http.StatusUnprocessableEntity, http.StatusUnprocessableEntity}, output.ExitUserError},
		"server error":       {[]int{http.StatusInternalServerError}, output.ExitInternalError},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			inGitRepoWithRemote(t, launchProjectRemote)
			api := &launchProjectAPI{createStatus: tc.statuses}
			client := api.start(t, "[]")
			env, _, _ := testLaunchEnvelope()

			_, created, err := launchEnsureProject(t.Context(), env, launchConfig{branch: "main"}, client, launchProjectRemote)

			require.Error(t, err)
			assert.False(t, created)
			var le *launchError
			require.True(t, isLaunchErr(err, &le), "got %v", err)
			assert.Equal(t, reasonProjectCreateFailed, le.Reason)
			assert.Equal(t, tc.exitCode, le.ExitCode())
			assert.Contains(t, le.Message, `"my-app"`)
			assert.Contains(t, le.NextStep, "dhq projects create")
			assert.Empty(t, api.repoURLs)
		})
	}
}

func dryRunWouldFrom(t *testing.T, stdout string) map[string]any {
	t.Helper()
	var resp struct {
		Data struct {
			Would    map[string]any `json:"would"`
			Requires []string       `json:"requires"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal([]byte(stdout), &resp), stdout)
	return resp.Data.Would
}

func TestLaunchDryRun_EmptyAccountSaysAProjectWouldBeCreated(t *testing.T) {
	inGitRepoWithRemote(t, launchProjectRemote)
	api := &launchProjectAPI{}
	client := api.start(t, "[]")
	env, stdout, _ := testLaunchEnvelopeJSON()

	require.NoError(t, launchDryRun(t.Context(), env, launchConfig{targetProtocol: "static_hosting"}, client, &sdk.AccountCapabilities{BetaFeatures: true}))

	would := dryRunWouldFrom(t, stdout.String())
	assert.Equal(t, true, would["create_project"])
	assert.Equal(t, "my-app", would["project_name"])
	assert.NotContains(t, would, "project")
	assert.Empty(t, api.created, "a dry run creates nothing")
	assert.Empty(t, api.repoURLs)
}

func TestLaunchDryRun_HumanOutputNamesTheProjectToCreate(t *testing.T) {
	inGitRepoWithRemote(t, launchProjectRemote)
	api := &launchProjectAPI{}
	client := api.start(t, "[]")
	env, _, stderr := testLaunchEnvelope()
	env.IsTTY = true

	require.NoError(t, launchDryRun(t.Context(), env, launchConfig{targetProtocol: "static_hosting"}, client, &sdk.AccountCapabilities{BetaFeatures: true}))

	assert.Contains(t, stderr.String(), `would create project "my-app"`)
}

func TestLaunchDryRun_MatchingProjectIsReported(t *testing.T) {
	inGitRepoWithRemote(t, launchProjectRemote)
	api := &launchProjectAPI{}
	client := api.start(t, launchProjectList(
		launchProjectJSON("Other", "other", "git@github.com:acme/other.git"),
		launchProjectJSON("Site", "site", launchProjectRemote),
	))
	env, stdout, _ := testLaunchEnvelopeJSON()

	require.NoError(t, launchDryRun(t.Context(), env, launchConfig{targetProtocol: "static_hosting"}, client, &sdk.AccountCapabilities{BetaFeatures: true}))

	would := dryRunWouldFrom(t, stdout.String())
	assert.Equal(t, "site", would["project"])
	assert.NotContains(t, would, "create_project")
}

func TestLaunchDryRun_RefusesTwoMatchingProjectsLikeTheLaunch(t *testing.T) {
	inGitRepoWithRemote(t, launchProjectRemote)
	api := &launchProjectAPI{}
	client := api.start(t, launchProjectList(
		launchProjectJSON("One", "one", launchProjectRemote),
		launchProjectJSON("Two", "two", launchProjectRemote),
	))
	env, _, _ := testLaunchEnvelopeJSON()

	err := launchDryRun(t.Context(), env, launchConfig{targetProtocol: "static_hosting"}, client, &sdk.AccountCapabilities{BetaFeatures: true})

	var le *launchError
	require.True(t, isLaunchErr(err, &le), "got %v", err)
	assert.Equal(t, reasonProjectAmbiguous, le.Reason)
}

func TestLaunchDryRun_WithoutAGitRemoteRequiresOne(t *testing.T) {
	t.Chdir(t.TempDir())
	api := &launchProjectAPI{}
	client := api.start(t, "[]")
	env, stdout, _ := testLaunchEnvelopeJSON()

	require.NoError(t, launchDryRun(t.Context(), env, launchConfig{targetProtocol: "static_hosting"}, client, &sdk.AccountCapabilities{BetaFeatures: true}))

	assert.Contains(t, stdout.String(), "git remote")
	assert.NotContains(t, dryRunWouldFrom(t, stdout.String()), "create_project")
}

// The whole command, as an agent runs it in an empty account.
func TestLaunchCommand_DryRunInEmptyAccountCreatesNothing(t *testing.T) {
	withResolvableContext(t)
	t.Setenv("DEPLOYHQ_PROJECT", "")
	t.Setenv("HOME", t.TempDir())
	inGitRepoWithRemote(t, launchProjectRemote)

	var (
		mu    sync.Mutex
		posts []string
	)
	orig := http.DefaultTransport
	http.DefaultTransport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		mu.Lock()
		defer mu.Unlock()
		if r.Method == http.MethodPost && !strings.Contains(r.URL.Path, "detect") {
			posts = append(posts, r.URL.Path)
		}
		body := `{}`
		if r.URL.Path == "/projects" {
			body = `[]`
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(body)),
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Request:    r,
		}, nil
	})
	t.Cleanup(func() { http.DefaultTransport = orig })

	stdout, err := runCmdCapturingStdout(t, "launch", "--static", "--custom-domain", "example.com", "--dry-run", "--non-interactive", "--json")

	require.NoError(t, err)
	would := dryRunWouldFrom(t, stdout)
	assert.Equal(t, true, would["create_project"], stdout)
	assert.Equal(t, "my-app", would["project_name"])
	assert.Empty(t, posts, "a dry run writes nothing")
}
