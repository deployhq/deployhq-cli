package skillinstaller

import (
	"fmt"
	"os"
	"path/filepath"
)

// safeWriteFile is os.WriteFile with a symlink-refusal Lstat check on both
// path and its immediate parent directory. If either already exists as a
// symlink, the call returns an error rather than following it.
//
// Threat model: a local actor pre-plants a symlink at one of dhq's
// predictable install targets (~/.codex/AGENTS.md, .github/copilot-instructions.md,
// ~/.aider/deployhq-skill.md, …) pointing at a victim file (e.g. ~/.bashrc).
// Without this check, our subsequent os.WriteFile follows the symlink and
// corrupts the victim. This is self-LPE, not a network threat — but cheap
// to close given how predictable our install paths are.
//
// The parent check closes the next hop: planting a symlink at the predictable
// *directory* we write into (e.g. ~/.codex or .github) redirects the leaf
// write even when the leaf itself isn't a symlink. We only check the immediate
// parent — deeper ancestors are out of scope under the local-only model, and
// walking to the filesystem root would false-positive on legitimately
// symlinked homes and macOS temp roots (/var → /private/var).
//
// Not race-free against an attacker who swaps in a symlink between the
// Lstat and the WriteFile. That window is irrelevant under the local-only
// threat model. Where the OS supports it, a follow-up could open with
// O_NOFOLLOW for full closure; the Lstat approach is portable and good
// enough for the defense-in-depth goal here.
func safeWriteFile(path string, data []byte, perm os.FileMode) error {
	if info, err := os.Lstat(path); err == nil {
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("refusing to write through symlink: %s", path)
		}
	}
	if err := ensureNotSymlinkDir(filepath.Dir(path)); err != nil {
		return err
	}
	return os.WriteFile(path, data, perm)
}

// ensureNotSymlinkDir returns an error if path exists and is a symlink. A
// missing path is fine — the caller is about to create it. Used by
// writeEmbeddedTree to refuse a symlinked destination root before walking
// and writing into it; without this, a planted ~/.codex/deployhq-references
// → /tmp/attacker symlink would redirect every reference file dhq writes.
func ensureNotSymlinkDir(path string) error {
	info, err := os.Lstat(path)
	if err != nil {
		return nil
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("refusing to write into symlinked directory: %s", path)
	}
	return nil
}
