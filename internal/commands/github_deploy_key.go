package commands

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
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

// isSSHRepoURL reports whether repoURL is an SSH-style git URL
// (git@host:owner/repo or ssh://...). DeployHQ checks it can read such a
// repository with the project's deploy key the moment the repository is
// connected, so the key has to be on the git host first. HTTP(S) URLs are not
// checked at connect time.
func isSSHRepoURL(repoURL string) bool {
	return strings.HasPrefix(repoURL, "git@") || strings.HasPrefix(repoURL, "ssh://")
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
