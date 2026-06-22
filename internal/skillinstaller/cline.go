package skillinstaller

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/deployhq/deployhq-cli/skills"
)

func init() { Register(&cline{}) }

// cline installs the skill as a project-level Cline rule.
//
// Layout: <repo>/.clinerules/deployhq.md
//
// Cline's project rules support two shapes: a single-file `.clinerules`
// (legacy) or a `.clinerules/` directory of *.md files (newer, preferred).
// We always write the directory form. If the user has the file form, we
// refuse with a clear migration hint rather than silently destroying it.
//
// Like Copilot, this is ScopeProject — modifying a user's repo as a side
// effect of `dhq hello` would be hostile. Opt in via:
//
//	dhq skills install --agent cline
type cline struct{}

func (cline) Name() string        { return "cline" }
func (cline) DisplayName() string { return "Cline" }
func (cline) Scope() Scope        { return ScopeProject }

const (
	clineRulesDir  = ".clinerules"
	clineSkillFile = "deployhq.md"
)

// inRepo mirrors copilot's ancestor-walk: any .git/ in the cwd or an
// ancestor means we're inside a project where installing makes sense.
func (c cline) inRepo() bool {
	dir, err := getCwd()
	if err != nil {
		return false
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
			return true
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return false
		}
		dir = parent
	}
}

// skillFile returns <cwd>/.clinerules/deployhq.md.
func (c cline) skillFile() (string, error) {
	cwd, err := getCwd()
	if err != nil {
		return "", err
	}
	return filepath.Join(cwd, clineRulesDir, clineSkillFile), nil
}

// rulesPathStat returns info about <cwd>/.clinerules so callers can tell
// the difference between "absent", "directory" (ours), and "file" (legacy
// single-file form, which would conflict with our directory writes).
func (c cline) rulesPathStat() (os.FileInfo, error) {
	cwd, err := getCwd()
	if err != nil {
		return nil, err
	}
	return os.Stat(filepath.Join(cwd, clineRulesDir))
}

func (c cline) Detect() Status {
	if !c.inRepo() {
		return StatusNotInstalled
	}

	// If .clinerules exists as a file (legacy shape), we can't safely
	// proceed. Report Available so it shows up in listings, and let
	// Install() fail loudly with an actionable message.
	if info, err := c.rulesPathStat(); err == nil && !info.IsDir() {
		return StatusAvailable
	}

	p, err := c.skillFile()
	if err != nil {
		return StatusNotInstalled
	}
	data, err := os.ReadFile(p)
	if err != nil {
		return StatusAvailable
	}
	switch parseOwnedFileVersion(string(data)) {
	case "":
		return StatusAvailable
	case skills.Version:
		return StatusInstalled
	default:
		return StatusOutdated
	}
}

func (c cline) Install() (string, error) {
	if !c.inRepo() {
		cwd, _ := getCwd()
		return "", fmt.Errorf("not a git repository (cwd=%s); run from inside a repo", cwd)
	}
	if info, err := c.rulesPathStat(); err == nil && !info.IsDir() {
		return "", fmt.Errorf(
			"%s exists as a file (legacy Cline single-rule shape); "+
				"move its contents into %s/main.md (or any *.md name), delete the file, "+
				"then re-run `dhq skills install --agent cline`",
			clineRulesDir, clineRulesDir,
		)
	}

	p, err := c.skillFile()
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return "", err
	}

	body, err := buildOwnedSkillFile(skills.FS, "deployhq")
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(p, body, 0o644); err != nil {
		return "", err
	}
	return p, nil
}
