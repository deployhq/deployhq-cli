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
// creating directories as needed. Existing files are overwritten — this is
// the simplest definition of idempotent and matches what users expect from
// "re-install" or "upgrade".
//
// Refuses when dst is a symlink: otherwise a planted symlink at the refs
// root would silently redirect every write into a victim directory.
// Per-file writes go through safeWriteFile, which adds the same check on
// the final path.
func writeEmbeddedTree(efs fs.FS, srcRoot, dst string) error {
	if err := ensureNotSymlinkDir(dst); err != nil {
		return err
	}
	return fs.WalkDir(efs, srcRoot, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel := strings.TrimPrefix(p, srcRoot)
		rel = strings.TrimPrefix(rel, "/")
		out := filepath.Join(dst, rel)

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
