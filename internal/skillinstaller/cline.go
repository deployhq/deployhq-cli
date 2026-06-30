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

func (c cline) Detect() Status {
	root, ok := findRepoRoot()
	if !ok {
		return StatusNotInstalled
	}

	// If .clinerules exists as a file at the repo root (legacy shape),
	// we can't safely proceed. Report Available so it shows up in
	// listings, and let Install() fail loudly with an actionable message.
	if info, err := os.Stat(filepath.Join(root, clineRulesDir)); err == nil && !info.IsDir() {
		return StatusAvailable
	}

	data, err := os.ReadFile(filepath.Join(root, clineRulesDir, clineSkillFile))
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
	root, ok := findRepoRoot()
	if !ok {
		cwd, _ := getCwd()
		return "", fmt.Errorf("not a git repository (cwd=%s); run from inside a repo", cwd)
	}
	if info, err := os.Stat(filepath.Join(root, clineRulesDir)); err == nil && !info.IsDir() {
		return "", fmt.Errorf(
			"%s exists as a file (legacy Cline single-rule shape); "+
				"move its contents into %s/main.md (or any *.md name), delete the file, "+
				"then re-run `dhq skills install --agent cline`",
			clineRulesDir, clineRulesDir,
		)
	}

	p := filepath.Join(root, clineRulesDir, clineSkillFile)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return "", err
	}

	body, err := buildOwnedSkillFile(skills.FS, "deployhq")
	if err != nil {
		return "", err
	}
	if err := writeOwnedFile(p, body, 0o644); err != nil {
		return "", err
	}
	return p, nil
}
