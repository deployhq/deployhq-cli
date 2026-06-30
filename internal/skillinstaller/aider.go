package skillinstaller

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/deployhq/deployhq-cli/skills"
)

func init() { Register(&aider{}) }

// aider installs the skill into a file Aider can read on demand.
//
// Aider doesn't auto-discover any file by name — unlike Copilot's
// .github/copilot-instructions.md or Claude's skills/ directory, every
// file Aider reads must be explicitly listed in .aider.conf.yml's `read:`
// directive or passed as `--read FILE` on the command line.
//
// Auto-editing .aider.conf.yml safely is hard (YAML doesn't have nestable
// block comments; the user may already have a `read:` key in a way that
// collides with our markers). So we install the skill file, then surface
// a PostInstallNote with the exact line to add. One manual step, zero
// risk of corrupting the user's config.
//
// Layout:
//   - ~/.aider/deployhq-skill.md
//     A single self-contained markdown file with a `<!-- deployhq-skill v1 -->`
//     comment at the top for version tracking. Aider reads this as one
//     conventions file; the version comment is invisible to the agent
//     but lets Detect() know when to mark an install outdated.
//
// Detection uses `aider` on PATH because ~/.aider/ may not exist before
// any explicit setup, and config files are optional.
type aider struct{}

func (aider) Name() string        { return "aider" }
func (aider) DisplayName() string { return "Aider" }
func (aider) Scope() Scope        { return ScopeUser }

const (
	aiderSkillDir  = ".aider"
	aiderSkillFile = "deployhq-skill.md"
)

// findAider is the binary-on-PATH lookup. Overridable in tests so they
// don't depend on whether the dev box actually has aider installed.
//
// Tests using this var must run serially — see the note on homeDir in
// claude.go for why this package forbids t.Parallel().
var findAider = func() bool {
	_, err := exec.LookPath("aider")
	return err == nil
}

func (a aider) skillDir() (string, error) {
	home, err := homeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, aiderSkillDir), nil
}

func (a aider) skillPath() (string, error) {
	dir, err := a.skillDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, aiderSkillFile), nil
}

func (a aider) Detect() Status {
	if !findAider() {
		return StatusNotInstalled
	}
	p, err := a.skillPath()
	if err != nil {
		return StatusNotInstalled
	}
	data, err := os.ReadFile(p)
	if err != nil {
		return StatusAvailable
	}
	switch parseOwnedFileVersion(string(data)) {
	case "":
		// File exists but with no version marker — treat as available so
		// the user gets a fresh install with proper versioning. We can't
		// safely assume an unmarked file is ours.
		return StatusAvailable
	case skills.Version:
		return StatusInstalled
	default:
		return StatusOutdated
	}
}

func (a aider) Install() (string, error) {
	dir, err := a.skillDir()
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}

	body, err := buildOwnedSkillFile(skills.FS, "deployhq")
	if err != nil {
		return "", err
	}

	dst := filepath.Join(dir, aiderSkillFile)
	if err := safeWriteFile(dst, body, 0o644); err != nil {
		return "", err
	}
	return dst, nil
}

// PostInstallNote tells the user how to wire the skill into Aider, since
// we can't safely auto-edit .aider.conf.yml. Surfaced by both the hello
// hook and `dhq skills install` via the Noter interface.
//
// The path is quoted per-context so each snippet is safe to paste verbatim:
// the YAML form (read: ["..."]) and the shell form (--read '...') use
// different quoting rules, since a double-quoted shell argument would still
// evaluate $(...), backticks, and $VAR. Using a single escaping for both
// would break on edge-case home paths.
func (a aider) PostInstallNote() string {
	p, err := a.skillPath()
	if err != nil {
		return ""
	}
	yamlPath := quotePathForYAML(p)
	shellPath := quotePathForShell(p)
	return fmt.Sprintf(
		"To load on every Aider run: add `read: [%s]` to ~/.aider.conf.yml "+
			"(or pass `--read %s` ad-hoc).",
		yamlPath, shellPath,
	)
}

// quotePathForYAML wraps a path in a YAML double-quoted scalar with internal
// backslashes and double quotes escaped.
func quotePathForYAML(p string) string {
	p = strings.ReplaceAll(p, `\`, `\\`)
	p = strings.ReplaceAll(p, `"`, `\"`)
	return `"` + p + `"`
}

// quotePathForShell returns a single-quoted POSIX shell literal. Single
// quotes suppress all expansion ($(...), backticks, $VAR), so the only
// character needing care is the single quote itself.
func quotePathForShell(p string) string {
	return `'` + strings.ReplaceAll(p, `'`, `'"'"'`) + `'`
}

