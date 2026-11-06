package commands

import (
	"encoding/json"
	"errors"
	"fmt"
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
	// branchesBody is the branch list; empty means main and develop.
	branchesBody string
	updateStatus int // status for updating the repository; 0 means 200
	// showBody is the body of GET /projects/my-app/repository.
	showBody string
	// sentBranch records the branch the CLI asked for when creating the
	// repository.
	sentBranch string
	// savedURL is the URL of the saved repository; empty means an SSH URL.
	savedURL string
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
		url := api.savedURL
		if url == "" {
			url = "git@github.com:acme/my-app.git"
		}
		saved := `{"scm_type":"git","url":"` + url + `","branch":"` + api.savedBranch + `"}`
		switch {
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/projects/my-app/repository"):
			var sent struct {
				Repository struct {
					Branch string `json:"branch"`
				} `json:"repository"`
			}
			if err := json.NewDecoder(r.Body).Decode(&sent); err != nil {
				return nil, err
			}
			api.sentBranch = sent.Repository.Branch
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
			if api.branchesBody != "" {
				return respond(http.StatusOK, api.branchesBody)
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
	api := &fakeRepoAPI{savedBranch: "main"}
	installFakeRepoAPI(t, api)

	err := runServersCmd(t, "repos", "create", "--scm-type", "git",
		"--url", "git@github.com:acme/my-app.git", "--branch", "does-not-exist")
	require.Error(t, err)
	assert.Equal(t, "does-not-exist", api.sentBranch, "the requested branch is sent to DeployHQ")

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
	api := &fakeRepoAPI{savedBranch: "main"}
	installFakeRepoAPI(t, api)

	require.NoError(t, runServersCmd(t, "repos", "create", "--scm-type", "git",
		"--url", "git@github.com:acme/my-app.git", "--branch", "main"))
	assert.Equal(t, "main", api.sentBranch, "the requested branch is sent to DeployHQ")
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

// A repository with no commits has no branches, so DeployHQ saves no branch:
// the advice is to push a commit, not to pick another branch.
func TestReposBranch_RepositoryWithNoBranchesYet(t *testing.T) {
	for _, tc := range []struct {
		args []string
		done string
	}{
		{[]string{"repos", "create", "--scm-type", "git", "--url", "git@github.com:acme/my-app.git", "--branch", "main"}, "connected"},
		{[]string{"repos", "update", "--branch", "main"}, "updated"},
	} {
		t.Run(tc.done, func(t *testing.T) {
			withResolvableContext(t)
			installFakeRepoAPI(t, &fakeRepoAPI{savedBranch: ""})

			err := runServersCmd(t, tc.args...)
			var uerr *output.UserError
			require.True(t, errors.As(err, &uerr), "got %T: %v", err, err)
			assert.Equal(t, output.ExitUserError, output.ClassifyError(err))
			assert.Equal(t, `The repository was `+tc.done+`, but it has no branches yet, so "main" couldn't be set`, uerr.Message)
			assert.Contains(t, uerr.Hint, "Push a commit to the repository, then run: dhq repos update -p my-app --branch main")
			assert.NotContains(t, uerr.Hint, "Pick one")

			data := output.ErrorResponseFromErr(err).Data.(output.ErrorData)
			require.NotEmpty(t, data.Recovery)
			assert.Equal(t, output.Breadcrumb{Action: "set_branch", Cmd: "dhq repos update -p my-app --branch main"}, data.Recovery[0])
		})
	}
}

func TestReposUpdate_LongBranchListIsCapped(t *testing.T) {
	withResolvableContext(t)
	var body []string
	for i := 0; i < 25; i++ {
		body = append(body, fmt.Sprintf(`"b%02d":"abc"`, i))
	}
	installFakeRepoAPI(t, &fakeRepoAPI{savedBranch: "b00", branchesBody: "{" + strings.Join(body, ",") + "}"})

	err := runServersCmd(t, "repos", "update", "--branch", "mian")
	var uerr *output.UserError
	require.True(t, errors.As(err, &uerr), "got %T: %v", err, err)
	assert.Contains(t, uerr.Hint, "Branches: b00, b01, ")
	assert.Contains(t, uerr.Hint, "b19 … and 5 more (see: dhq repos branches -p my-app).")
	assert.NotContains(t, uerr.Hint, "b20")
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

// DeployHQ doesn't check the branch of a git repository connected over
// HTTP(S): it saves the branch as given. dhq checks it against the branch list.
func TestReposBranch_HTTPSBranchMissingFails(t *testing.T) {
	const httpsURL = "https://github.com/acme/my-app.git"
	for _, tc := range []struct {
		args []string
		done string
	}{
		{[]string{"repos", "create", "--scm-type", "git", "--url", httpsURL, "--branch", "does-not-exist"}, "connected"},
		{[]string{"repos", "update", "--branch", "does-not-exist"}, "updated"},
	} {
		t.Run(tc.done, func(t *testing.T) {
			withResolvableContext(t)
			installFakeRepoAPI(t, &fakeRepoAPI{savedBranch: "does-not-exist", savedURL: httpsURL})

			err := runServersCmd(t, tc.args...)
			var uerr *output.UserError
			require.True(t, errors.As(err, &uerr), "got %T: %v", err, err)
			assert.Equal(t, output.ExitUserError, output.ClassifyError(err))
			assert.Equal(t, `The repository was `+tc.done+` on branch "does-not-exist", but "does-not-exist" isn't in it`, uerr.Message)
			assert.Contains(t, uerr.Hint, "Branches: develop, main.")
			assert.Contains(t, uerr.Hint, "Pick one with: dhq repos update -p my-app --branch <name>")

			data := output.ErrorResponseFromErr(err).Data.(output.ErrorData)
			require.Len(t, data.Recovery, 2, "no deploy key for an HTTPS repository: %+v", data.Recovery)
			assert.Equal(t, "set_branch", data.Recovery[0].Action)
			assert.Equal(t, "list_branches", data.Recovery[1].Action)
		})
	}
}

func TestReposBranch_HTTPSBranchPresentSucceeds(t *testing.T) {
	withResolvableContext(t)
	installFakeRepoAPI(t, &fakeRepoAPI{savedBranch: "develop", savedURL: "https://github.com/acme/my-app.git"})

	require.NoError(t, runServersCmd(t, "repos", "create", "--scm-type", "git",
		"--url", "https://github.com/acme/my-app.git", "--branch", "develop"))
}

// When the branch list can't be read (a private repository whose credentials
// don't work yet, a network error), the branch can't be shown to be wrong.
func TestReposBranch_HTTPSBranchListUnavailableSucceeds(t *testing.T) {
	withResolvableContext(t)
	installFakeRepoAPI(t, &fakeRepoAPI{
		savedBranch:    "does-not-exist",
		savedURL:       "https://github.com/acme/my-app.git",
		branchesStatus: http.StatusInternalServerError,
	})

	require.NoError(t, runServersCmd(t, "repos", "update", "--branch", "does-not-exist"))
}

func TestIsHTTPRepoURL(t *testing.T) {
	for url, want := range map[string]bool{
		"https://github.com/acme/app.git":   true,
		"HTTP://git.example.com/app.git":    true,
		"git@github.com:acme/app.git":       false,
		"ssh://git@github.com/acme/app.git": false,
		"svn+ssh://svn.example.com/app":     false,
	} {
		assert.Equal(t, want, isHTTPRepoURL(url), url)
	}
}
