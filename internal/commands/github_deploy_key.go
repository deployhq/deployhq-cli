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
