package commands

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"strings"

	"github.com/deployhq/deployhq-cli/pkg/sdk"
)

// ghAvailable reports whether the GitHub CLI (`gh`) is installed on PATH.
func ghAvailable() bool {
	_, err := exec.LookPath("gh")
	return err == nil
}

// installDeployKeyViaGH adds publicKey as a read-only deploy key to the GitHub
// repo identified by repoURL, using the local `gh` CLI (which carries its own
// authentication — so this works headlessly, with no browser or prompt).
//
// It returns an error when the URL isn't a GitHub repo, the key can't be
// written, or `gh` fails (e.g. not authenticated, no write access, or a key
// with the same title already exists). Callers treat a failure as non-fatal and
// fall back to surfacing the key for manual installation.
func installDeployKeyViaGH(repoURL, publicKey, title string) error {
	repo := extractGitHubRepo(repoURL)
	if repo == "" {
		return fmt.Errorf("could not extract a GitHub repo from URL: %s", repoURL)
	}

	tmpFile, err := os.CreateTemp("", "dhq-deploy-key-*.pub")
	if err != nil {
		return err
	}
	defer os.Remove(tmpFile.Name()) //nolint:errcheck

	if _, err := tmpFile.WriteString(publicKey); err != nil {
		tmpFile.Close() //nolint:errcheck
		return err
	}
	tmpFile.Close() //nolint:errcheck

	cmd := exec.Command("gh", "repo", "deploy-key", "add", tmpFile.Name(),
		"--repo", repo,
		"--title", title)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("%s", strings.TrimSpace(string(out)))
	}
	return nil
}

// isSSHRepoURL reports whether repoURL is an SSH-style git URL: ssh://...,
// or git's scp-like user@host:path form with any user name (git@github.com:
// owner/repo, deploy@git.example.com:org/repo.git). DeployHQ checks it can
// read such a repository with the project's deploy key the moment the
// repository is connected, so the key has to be on the git host first.
// HTTP(S) URLs are not checked at connect time.
func isSSHRepoURL(repoURL string) bool {
	if strings.HasPrefix(repoURL, "ssh://") {
		return true
	}
	if strings.Contains(repoURL, "://") {
		return false
	}
	userHost, _, found := strings.Cut(repoURL, ":")
	return found && strings.Contains(userHost, "@") && !strings.Contains(userHost, "/")
}

// githubDeployKeySettingsURL returns the GitHub page where a deploy key can be
// added for repoURL, or "" when repoURL is not a GitHub repository.
func githubDeployKeySettingsURL(repoURL string) string {
	if !isGitHubURL(repoURL) {
		return ""
	}
	repo := extractGitHubRepo(repoURL)
	if repo == "" {
		return ""
	}
	return fmt.Sprintf("https://github.com/%s/settings/keys/new", repo)
}

// deployKeyInstructions explains how to add publicKey (the project's PUBLIC
// deploy key, safe to display) to the repository host so DeployHQ can read
// repoURL, and which command to run afterwards. It is shown when connecting a
// repository fails because the key is not on the host yet.
func deployKeyInstructions(repoURL, publicKey, rerun string) string {
	var b strings.Builder
	b.WriteString("Add the project's public deploy key to the repository (read-only access is enough), then connect it again.\n\n")
	fmt.Fprintf(&b, "Public deploy key:\n  %s\n\n", publicKey)
	if settings := githubDeployKeySettingsURL(repoURL); settings != "" {
		fmt.Fprintf(&b, "Paste it at: %s\n", settings)
		fmt.Fprintf(&b, "Or with the GitHub CLI: echo '%s' | gh repo deploy-key add -R %s --title DeployHQ -\n", publicKey, extractGitHubRepo(repoURL))
	} else {
		b.WriteString("Add it in your git host's repository settings, under Deploy keys (Bitbucket: Access keys).\n")
	}
	if rerun != "" {
		fmt.Fprintf(&b, "\nThen run: %s", rerun)
	}
	return strings.TrimRight(b.String(), "\n")
}

// launchRepoFailure turns a repository-connection failure into the terminal
// launch error, keeping one that launchEnsureRepo already built.
func launchRepoFailure(err error) error {
	var le *launchError
	if errors.As(err, &le) {
		return le
	}
	return &launchError{
		Reason:   reasonRepoUnreachable,
		Message:  "Could not connect repository to project: " + err.Error(),
		NextStep: "Verify the repository URL is accessible and retry, or connect it manually in the DeployHQ dashboard.",
	}
}

// launchNewProjectRepoFailure is launchRepoFailure for a project this run just
// created. Re-running launch without --project would create a second project,
// so every failure says to reuse this one, as the deploy-key error already does.
func launchNewProjectRepoFailure(err error, projectID string) error {
	wrapped := launchRepoFailure(err)
	var le *launchError
	if !errors.As(wrapped, &le) || le.Details["project"] != "" {
		return wrapped // the deploy-key error already names the project
	}
	le.NextStep = fmt.Sprintf("Verify the repository URL is accessible, then run the same dhq launch command with --project %s added, or connect it manually in the DeployHQ dashboard.", projectID)
	if le.Details == nil {
		le.Details = map[string]string{}
	}
	le.Details["project"] = projectID
	return le
}

// isRepoAccessRejected reports whether err is DeployHQ refusing a repository
// because it could not read it with the configured authentication: for an SSH
// URL, the deploy key is not on the git host yet. Other failures (timeouts,
// server errors, other validation errors) are not, and must keep their own
// message, exit code and retryability.
func isRepoAccessRejected(err error) bool {
	var apiErr *sdk.APIError
	if !errors.As(err, &apiErr) || apiErr.StatusCode != http.StatusUnprocessableEntity {
		return false
	}
	for _, msg := range append([]string{apiErr.Message}, apiErr.Errors...) {
		if strings.Contains(strings.ToLower(msg), "using the authentication method provided") {
			return true
		}
	}
	return false
}

// launchRepoKeyError explains a rejected SSH repository: the project's deploy
// key is not on the git host yet. It carries the public key, where to add it,
// and the command that resumes launch with the same project.
func launchRepoKeyError(ctx context.Context, client *sdk.Client, projectID string, proj *sdk.Project, gitRemote string, err error) error {
	if !isRepoAccessRejected(err) {
		return launchRepoFailure(err)
	}
	publicKey := ""
	if proj != nil {
		publicKey = proj.PublicKey
	}
	if publicKey == "" {
		if fresh, getErr := client.GetProject(ctx, projectID); getErr == nil && fresh != nil {
			publicKey = fresh.PublicKey
		}
	}
	if publicKey == "" {
		return launchRepoFailure(err)
	}
	return &launchError{
		Reason:   reasonRepoUnreachable,
		Message:  "Could not connect repository to project: " + err.Error(),
		NextStep: deployKeyInstructions(gitRemote, publicKey, fmt.Sprintf("the same dhq launch command with --project %s added", projectID)),
		Details:  map[string]string{"project": projectID, "public_key": publicKey},
	}
}

// deployKeyPausePrompt is the interactive prompt shown while waiting for the
// user to add the deploy key. Over SSH the key is needed even for a public
// repository (the git host refuses unknown keys), so only HTTP(S) repositories
// may skip it.
func deployKeyPausePrompt(repoURL string) string {
	if isSSHRepoURL(repoURL) {
		return "Press Enter once the key is added (needed over SSH even for a public repository)... "
	}
	return "Press Enter once the key is added (or now, if the repo is public)... "
}
