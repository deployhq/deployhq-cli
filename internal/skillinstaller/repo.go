package skillinstaller

import (
	"os"
	"path/filepath"
)

// getCwd is the working-directory lookup. Overridable in tests so they
// don't depend on the dev box's real cwd. Project-scope targets use this
// as the starting point for findRepoRoot.
//
// Tests using this var must run serially — see the note on homeDir in
// claude.go for why this package forbids t.Parallel().
var getCwd = os.Getwd

// findRepoRoot walks up from the current working directory looking for a
// .git ancestor and returns (rootPath, true) when found. Project-scope
// targets must use this — never cwd — for both detection and writes, so
// that running `dhq` from a subdirectory still installs into the actual
// repo root (Copilot won't load `subdir/.github/copilot-instructions.md`).
func findRepoRoot() (string, bool) {
	dir, err := getCwd()
	if err != nil {
		return "", false
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
			return dir, true
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", false
		}
		dir = parent
	}
}
