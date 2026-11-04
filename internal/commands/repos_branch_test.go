package commands

import (
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/deployhq/deployhq-cli/internal/output"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeRepoAPI answers the repository endpoints of project my-app. The saved
// repository is on savedBranch whatever branch was asked for, as DeployHQ does
// for a branch that isn't in the repository.
type fakeRepoAPI struct {
	savedBranch    string
	branchesStatus int // status for the branch list; 0 means 200
	updateStatus   int // status for updating the repository; 0 means 200
	// showBody is the body of GET /projects/my-app/repository.
	showBody string
}

func installFakeRepoAPI(t *testing.T, api *fakeRepoAPI) {
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
		saved := `{"scm_type":"git","url":"git@github.com:acme/my-app.git","branch":"` + api.savedBranch + `"}`
		switch {
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/projects/my-app/repository"):
			return respond(http.StatusCreated, saved)
		case r.Method == http.MethodPut && strings.HasSuffix(r.URL.Path, "/projects/my-app/repository"):
			if api.updateStatus != 0 {
				return respond(api.updateStatus, `{"error":"Internal Server Error"}`)
			}
			return respond(http.StatusOK, saved)
		case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/projects/my-app/repository/branches"):
			if api.branchesStatus != 0 {
				return respond(api.branchesStatus, `{"error":"Internal Server Error"}`)
			}
			return respond(http.StatusOK, `{"main":"abc123","develop":"def456"}`)
		case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/projects/my-app/repository"):
			return respond(http.StatusOK, api.showBody)
		case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/projects/my-app"):
			return respond(http.StatusOK, `{"identifier":"p-1","name":"My App","permalink":"my-app","public_key":"`+projectPublicKey+`"}`)
		}
		// Anything else (e.g. the background update check) is irrelevant here.
		return respond(http.StatusOK, `{}`)
	})
	t.Cleanup(func() { http.DefaultTransport = orig })
}

func TestReposCreate_BranchReplacedByServerFails(t *testing.T) {
	withResolvableContext(t)
	installFakeRepoAPI(t, &fakeRepoAPI{savedBranch: "main"})

	err := runServersCmd(t, "repos", "create", "--scm-type", "git",
		"--url", "git@github.com:acme/my-app.git", "--branch", "does-not-exist")
	require.Error(t, err)

	var uerr *output.UserError
	require.True(t, errors.As(err, &uerr), "got %T: %v", err, err)
	assert.Equal(t, output.ExitUserError, output.ClassifyError(err))
	assert.Equal(t, `The repository was connected, but on branch "main": "does-not-exist" isn't in it`, uerr.Message)
	assert.Contains(t, uerr.Hint, "Branches: develop, main.")
	assert.Contains(t, uerr.Hint, "Pick one with: dhq repos update -p my-app --branch <name>")
	assert.Contains(t, uerr.Hint, "Nothing has been deployed.")

	data := output.ErrorResponseFromErr(err).Data.(output.ErrorData)
	require.GreaterOrEqual(t, len(data.Recovery), 2)
	assert.Equal(t, output.Breadcrumb{Action: "set_branch", Cmd: "dhq repos update -p my-app --branch <name>"}, data.Recovery[0])
	assert.Equal(t, output.Breadcrumb{Action: "list_branches", Cmd: "dhq repos branches -p my-app"}, data.Recovery[1])

	// The repository is connected, so the deploy key is still needed.
	keyCrumb := false
	for _, r := range data.Recovery {
		if strings.Contains(r.Cmd, "gh repo deploy-key add") && strings.Contains(r.Cmd, projectPublicKey) {
			keyCrumb = true
		}
	}
	assert.True(t, keyCrumb, "deploy key recovery kept: %+v", data.Recovery)
}

func TestReposCreate_BranchKeptSucceeds(t *testing.T) {
	withResolvableContext(t)
	installFakeRepoAPI(t, &fakeRepoAPI{savedBranch: "main"})

	require.NoError(t, runServersCmd(t, "repos", "create", "--scm-type", "git",
		"--url", "git@github.com:acme/my-app.git", "--branch", "main"))
}

func TestReposCreate_NoBranchGivenSucceeds(t *testing.T) {
	withResolvableContext(t)
	installFakeRepoAPI(t, &fakeRepoAPI{savedBranch: "main"})

	require.NoError(t, runServersCmd(t, "repos", "create", "--scm-type", "git",
		"--url", "git@github.com:acme/my-app.git"))
}

func TestReposUpdate_BranchReplacedByServerFails(t *testing.T) {
	withResolvableContext(t)
	installFakeRepoAPI(t, &fakeRepoAPI{savedBranch: "main"})

	err := runServersCmd(t, "repos", "update", "--branch", "mian")
	require.Error(t, err)

	var uerr *output.UserError
	require.True(t, errors.As(err, &uerr), "got %T: %v", err, err)
	assert.Equal(t, `The repository was updated, but on branch "main": "mian" isn't in it`, uerr.Message)
	assert.Contains(t, uerr.Hint, "Branches: develop, main.")

	data := output.ErrorResponseFromErr(err).Data.(output.ErrorData)
	require.Len(t, data.Recovery, 2)
	assert.Equal(t, "set_branch", data.Recovery[0].Action)
	assert.Equal(t, "list_branches", data.Recovery[1].Action)
}

func TestReposUpdate_BranchListUnavailablePointsAtBranchesCommand(t *testing.T) {
	withResolvableContext(t)
	installFakeRepoAPI(t, &fakeRepoAPI{savedBranch: "main", branchesStatus: http.StatusInternalServerError})

	err := runServersCmd(t, "repos", "update", "--branch", "mian")
	var uerr *output.UserError
	require.True(t, errors.As(err, &uerr), "got %T: %v", err, err)
	assert.NotContains(t, uerr.Hint, "Branches:")
	assert.Contains(t, uerr.Hint, "dhq repos branches -p my-app")
	assert.Contains(t, uerr.Hint, "dhq repos update -p my-app --branch <name>")
}

// DeployHQ answers a repository update for a project with no repository with
// a server error; reading the repository then returns a placeholder with no URL.
func TestReposUpdate_NoRepositoryYetPointsAtCreate(t *testing.T) {
	withResolvableContext(t)
	installFakeRepoAPI(t, &fakeRepoAPI{
		updateStatus: http.StatusInternalServerError,
		showBody:     `{"scm_type":"git","url":null,"branch":null}`,
	})

	err := runServersCmd(t, "repos", "update", "--branch", "main")
	var uerr *output.UserError
	require.True(t, errors.As(err, &uerr), "got %T: %v", err, err)
	assert.Equal(t, output.ExitUserError, output.ClassifyError(err))
	assert.Equal(t, "This project has no repository yet", uerr.Message)
	assert.Equal(t, "dhq repos create -p my-app --scm-type git --url <url> [--branch <branch>]", uerr.Hint)
}

// Newer DeployHQ servers answer the same update with a 404 instead.
func TestReposUpdate_NotFoundWithoutRepositoryPointsAtCreate(t *testing.T) {
	withResolvableContext(t)
	installFakeRepoAPI(t, &fakeRepoAPI{
		updateStatus: http.StatusNotFound,
		showBody:     `{"scm_type":"git","url":null,"branch":null}`,
	})

	err := runServersCmd(t, "repos", "update", "--branch", "main")
	var uerr *output.UserError
	require.True(t, errors.As(err, &uerr), "got %T: %v", err, err)
	assert.Equal(t, "This project has no repository yet", uerr.Message)
}

func TestReposUpdate_ServerErrorWithRepositoryKeepsOriginalError(t *testing.T) {
	withResolvableContext(t)
	installFakeRepoAPI(t, &fakeRepoAPI{
		updateStatus: http.StatusInternalServerError,
		showBody:     `{"scm_type":"git","url":"git@github.com:acme/my-app.git","branch":"main"}`,
	})

	err := runServersCmd(t, "repos", "update", "--branch", "main")
	require.Error(t, err)
	var uerr *output.UserError
	assert.False(t, errors.As(err, &uerr), "a server error on a project with a repository is not explained away")
	assert.Equal(t, output.ExitInternalError, output.ClassifyError(err))
	assert.Contains(t, err.Error(), "500")
}
