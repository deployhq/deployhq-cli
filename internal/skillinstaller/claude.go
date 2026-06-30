package skillinstaller

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/deployhq/deployhq-cli/skills"
)

func init() { Register(&claudeCode{}) }

// homeDir is the user home-directory lookup. Overridable in tests so we
// don't write into the dev's real home.
//
// Tests in this package MUST NOT call t.Parallel(): homeDir (along with
// getCwd in repo.go and findAider in aider.go) is mutable global state.
// Concurrent tests would race on these vars and produce flaky output. The
// install/detect paths are fast enough that serial execution costs nothing.
var homeDir = os.UserHomeDir

// claudeCode installs the skill into Claude Code's user-level skills directory.
//
// Layout: ~/.claude/skills/deployhq/SKILL.md + references/*.md
// Version marker: ~/.claude/skills/deployhq/.deployhq-skill-version (one line, schema version)
//
// We chose user-level (~/.claude/skills/) over project-level (.claude/skills/)
// because dhq hello is typically run once per machine, not once per project.
// Users who want project-scoped skills can copy the directory themselves.
type claudeCode struct{}

func (claudeCode) Name() string        { return "claude-code" }
func (claudeCode) DisplayName() string { return "Claude Code" }
func (claudeCode) Scope() Scope        { return ScopeUser }

// skillDirName is the on-disk directory name under each agent's skills root.
// Matches the `name:` field in skills/deployhq/SKILL.md frontmatter so Claude
// Code finds it via its standard discovery flow.
const skillDirName = "deployhq"

// versionMarker is the filename of the schema-version sentinel.
const versionMarker = ".deployhq-skill-version"

func (c claudeCode) configDir() (string, error) {
	home, err := homeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".claude"), nil
}

func (c claudeCode) skillDir() (string, error) {
	cfg, err := c.configDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(cfg, "skills", skillDirName), nil
}

func (c claudeCode) Detect() Status {
	cfg, err := c.configDir()
	if err != nil {
		return StatusNotInstalled
	}
	if _, err := os.Stat(cfg); err != nil {
		// No ~/.claude → Claude Code isn't installed (or has never run).
		return StatusNotInstalled
	}

	dir, err := c.skillDir()
	if err != nil {
		return StatusNotInstalled
	}
	if _, err := os.Stat(filepath.Join(dir, "SKILL.md")); err != nil {
		return StatusAvailable
	}

	// SKILL.md present — compare version markers.
	switch readVersion(filepath.Join(dir, versionMarker)) {
	case skills.Version:
		return StatusInstalled
	default:
		return StatusOutdated
	}
}

func (c claudeCode) Install() (string, error) {
	dir, err := c.skillDir()
	if err != nil {
		return "", err
	}
	if err := writeEmbeddedTree(skills.FS, "deployhq", dir); err != nil {
		return "", err
	}
	if err := safeWriteFile(filepath.Join(dir, versionMarker), []byte(skills.Version+"\n"), 0o644); err != nil {
		return "", err
	}
	return dir, nil
}

// writeEmbeddedTree copies srcRoot from the embedded FS into dst on disk,
// replacing any existing tree. This is the simplest definition of idempotent
// and matches what users expect from "re-install" or "upgrade".
//
// The new tree is staged in a temporary sibling directory and only swapped
// into place once it is fully written. A failure mid-write therefore leaves
// the existing install untouched rather than deleting it and leaving a
// half-written (or empty) tree behind — important because for project-scope
// targets this lives inside the user's repo.
//
// Refuses when dst is a symlink: otherwise a planted symlink at the refs
// root would silently redirect the swap into a victim directory. Per-file
// writes go through safeWriteFile, which adds the same check on the final
// path and its immediate parent.
func writeEmbeddedTree(efs fs.FS, srcRoot, dst string) error {
	if err := ensureNotSymlinkDir(dst); err != nil {
		return err
	}

	parent := filepath.Dir(dst)
	if err := os.MkdirAll(parent, 0o755); err != nil {
		return err
	}

	// Stage in a sibling temp dir so the swap is on the same filesystem
	// (os.Rename is then atomic) and the old tree survives a failed write.
	staging, err := os.MkdirTemp(parent, "."+filepath.Base(dst)+".tmp-")
	if err != nil {
		return err
	}
	committed := false
	defer func() {
		if !committed {
			_ = os.RemoveAll(staging)
		}
	}()
	// MkdirTemp creates 0700; the published tree should be world-readable.
	if err := os.Chmod(staging, 0o755); err != nil {
		return err
	}

	walkErr := fs.WalkDir(efs, srcRoot, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel := strings.TrimPrefix(p, srcRoot)
		rel = strings.TrimPrefix(rel, "/")
		out := filepath.Join(staging, rel)

		if d.IsDir() {
			return os.MkdirAll(out, 0o755)
		}
		data, err := fs.ReadFile(efs, p)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
			return err
		}
		return safeWriteFile(out, data, 0o644)
	})
	if walkErr != nil {
		return walkErr
	}

	// Swap the freshly-staged tree in. The old tree is removed only now,
	// after the new one is known-good.
	if err := os.RemoveAll(dst); err != nil {
		return err
	}
	if err := os.Rename(staging, dst); err != nil {
		return err
	}
	committed = true
	return nil
}

// readVersion returns the trimmed contents of the version marker, or empty
// string if the file is missing or unreadable.
func readVersion(path string) string {
	b, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(b))
}
