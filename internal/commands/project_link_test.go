package commands

import (
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/deployhq/deployhq-cli/internal/config"
	"github.com/deployhq/deployhq-cli/internal/output"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// projectsListServer serves GET /projects with body and fails the test on any
// other request (in particular, any attempt to create a project).
func projectsListServer(t *testing.T, body string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && r.URL.Path == "/projects" {
			_, _ = w.Write([]byte(body))
			return
		}
		t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	t.Cleanup(srv.Close)
	return srv
}

const twoProjectsJSON = `[
  {"name":"My App","permalink":"my-app","identifier":"p-1"},
  {"name":"Other","permalink":"other","identifier":"p-2"}
]`

func TestFindProjectsNamed_CaseInsensitive(t *testing.T) {
	srv := projectsListServer(t, twoProjectsJSON)
	matches, err := findProjectsNamed(t.Context(), newSpecValidatingClient(t, srv), "  my app ")
	require.NoError(t, err)
	require.Len(t, matches, 1)
	assert.Equal(t, "my-app", matches[0].Permalink)
}

// The server accepts duplicate names (it suffixes the permalink), so in
// non-interactive mode the CLI must refuse and list the existing project.
func TestConfirmNoDuplicateProject_NonInteractiveRefuses(t *testing.T) {
	srv := projectsListServer(t, twoProjectsJSON)
	env, _, _ := testLaunchEnvelope()

	err := confirmNoDuplicateProject(t.Context(), env, newSpecValidatingClient(t, srv), "My App", false)
	var userErr *output.UserError
	require.ErrorAs(t, err, &userErr)
	assert.Contains(t, userErr.Message, "My App (my-app)")
	assert.Contains(t, userErr.Hint, "dhq config set project my-app")
	assert.Contains(t, userErr.Hint, "--allow-duplicate")
}

func TestConfirmNoDuplicateProject_AllowDuplicateWarnsAndProceeds(t *testing.T) {
	srv := projectsListServer(t, twoProjectsJSON)
	env, _, stderr := testLaunchEnvelope()

	err := confirmNoDuplicateProject(t.Context(), env, newSpecValidatingClient(t, srv), "My App", true)
	require.NoError(t, err)
	assert.Contains(t, stderr.String(), "my-app")
}

func TestConfirmNoDuplicateProject_NoMatchProceeds(t *testing.T) {
	srv := projectsListServer(t, twoProjectsJSON)
	env, _, _ := testLaunchEnvelope()
	require.NoError(t, confirmNoDuplicateProject(t.Context(), env, newSpecValidatingClient(t, srv), "Brand New", false))
}

func TestConfirmNoDuplicateProject_InteractiveAsks(t *testing.T) {
	srv := projectsListServer(t, twoProjectsJSON)
	orig := confirmDuplicateProjectFn
	t.Cleanup(func() { confirmDuplicateProjectFn = orig })

	env, _, _ := testLaunchEnvelope()
	env.NonInteractive = false

	confirmDuplicateProjectFn = func(string) bool { return false }
	assert.Error(t, confirmNoDuplicateProject(t.Context(), env, newSpecValidatingClient(t, srv), "My App", false))

	confirmDuplicateProjectFn = func(string) bool { return true }
	assert.NoError(t, confirmNoDuplicateProject(t.Context(), env, newSpecValidatingClient(t, srv), "My App", false))
}

// A failed lookup must never block project creation.
func TestConfirmNoDuplicateProject_ListErrorDoesNotBlock(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()
	env, _, _ := testLaunchEnvelope()
	require.NoError(t, confirmNoDuplicateProject(t.Context(), env, newTestClient(t, srv), "My App", false))
}

func TestLinkProjectToDirectory_WritesWhenUnset(t *testing.T) {
	t.Chdir(t.TempDir())
	env, _, _ := testLaunchEnvelope()

	linkProjectToDirectory(env, "my-app")
	assert.Equal(t, "my-app", config.Get(config.ProjectConfigPath(), "project"))
}

// An existing link to a different project is never silently replaced.
func TestLinkProjectToDirectory_KeepsDifferentProject(t *testing.T) {
	t.Chdir(t.TempDir())
	require.NoError(t, os.WriteFile(".deployhq.toml", []byte("project = 'old-app'\naccount = 'acme'\n"), 0o644))
	env, _, stderr := testLaunchEnvelope()

	linkProjectToDirectory(env, "new-app")
	assert.Equal(t, "old-app", config.Get(config.ProjectConfigPath(), "project"))
	assert.Equal(t, "acme", config.Get(config.ProjectConfigPath(), "account"))
	assert.Contains(t, stderr.String(), "dhq config set project new-app")
}

// An empty project entry (as written by a bare 'dhq config init') is filled in,
// and other keys are preserved.
func TestLinkProjectToDirectory_FillsEmptyEntry(t *testing.T) {
	t.Chdir(t.TempDir())
	require.NoError(t, os.WriteFile(".deployhq.toml", []byte("project = ''\naccount = 'acme'\n"), 0o644))
	env, _, _ := testLaunchEnvelope()

	linkProjectToDirectory(env, "my-app")
	assert.Equal(t, "my-app", config.Get(config.ProjectConfigPath(), "project"))
	assert.Equal(t, "acme", config.Get(config.ProjectConfigPath(), "account"))
}

// The init wizard reports a taken name on the name step without creating
// anything; submitting the same name again creates it.
func TestInitWizard_DuplicateNameNeedsSecondSubmit(t *testing.T) {
	posts := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/projects":
			_, _ = w.Write([]byte(twoProjectsJSON))
		case r.Method == http.MethodPost && r.URL.Path == "/projects":
			posts++
			_, _ = w.Write([]byte(`{"name":"My App","permalink":"my-app-1","identifier":"p-3"}`))
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer srv.Close()

	env, _, stderr := testLaunchEnvelope()
	env.NonInteractive = false
	m := &initModel{step: stepProjectName, client: newTestClient(t, srv), env: env, projectName: "My App", input: "My App"}

	msg := m.createProject().(createResultMsg)
	require.Error(t, msg.err)
	assert.Contains(t, msg.err.Error(), "My App (my-app)")
	assert.Contains(t, msg.err.Error(), "Press Enter again")
	assert.Zero(t, posts, "must not create on the first submit")
	assert.Empty(t, stderr.String(), "must not write over the wizard")

	m.Update(msg)
	assert.Equal(t, stepProjectName, m.step)
	assert.Equal(t, "My App", m.input, "the name stays in the input for a resubmit")

	msg = m.createProject().(createResultMsg)
	require.NoError(t, msg.err)
	assert.Equal(t, 1, posts)
	m.Update(msg)
	assert.Equal(t, "my-app-1", m.project.Permalink)
}

// A free name is created on the first submit.
func TestInitWizard_FreeNameCreatesImmediately(t *testing.T) {
	posts := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			posts++
			_, _ = w.Write([]byte(`{"name":"New","permalink":"new","identifier":"p-4"}`))
			return
		}
		_, _ = w.Write([]byte(twoProjectsJSON))
	}))
	defer srv.Close()

	env, _, _ := testLaunchEnvelope()
	m := &initModel{step: stepProjectName, client: newTestClient(t, srv), env: env, projectName: "New"}
	msg := m.createProject().(createResultMsg)
	require.NoError(t, msg.err)
	assert.Equal(t, 1, posts)
}

// A .deployhq.toml that exists but can't be parsed is left exactly as it is:
// reading it as "no project" would overwrite the user's settings.
func TestLinkProjectToDirectory_LeavesUnparseableFileUntouched(t *testing.T) {
	t.Chdir(t.TempDir())
	broken := []byte("project = 'old-app\naccount = 'acme'\n[[[\n")
	require.NoError(t, os.WriteFile(".deployhq.toml", broken, 0o644))
	env, stdout, stderr := testLaunchEnvelope()

	linkProjectToDirectory(env, "my-app")

	after, err := os.ReadFile(".deployhq.toml")
	require.NoError(t, err)
	assert.Equal(t, broken, after, "an unreadable .deployhq.toml must not be rewritten")
	assert.Contains(t, stderr.String(), ".deployhq.toml")
	assert.Contains(t, stderr.String(), "dhq config set project my-app")
	assert.Empty(t, stdout.String(), "the warning goes to stderr only")
}
