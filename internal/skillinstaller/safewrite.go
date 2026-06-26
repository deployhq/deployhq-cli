package skillinstaller

import (
	"fmt"
	"os"
)

// safeWriteFile is os.WriteFile with a symlink-refusal Lstat check on path.
// If path already exists as a symlink, the call returns an error rather than
// following it.
//
// Threat model: a local actor pre-plants a symlink at one of dhq's
// predictable install targets (~/.codex/AGENTS.md, .github/copilot-instructions.md,
// ~/.aider/deployhq-skill.md, …) pointing at a victim file (e.g. ~/.bashrc).
// Without this check, our subsequent os.WriteFile follows the symlink and
// corrupts the victim. This is self-LPE, not a network threat — but cheap
// to close given how predictable our install paths are.
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
