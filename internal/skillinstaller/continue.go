package skillinstaller

import (
	"os"
	"path/filepath"

	"github.com/deployhq/deployhq-cli/skills"
)

func init() { Register(&continueDev{}) }

// continueDev installs the skill as a user-level Continue.dev rule.
//
// Layout:
//   - ~/.continue/rules/deployhq.md
//
// Newer Continue versions auto-load every *.md file under ~/.continue/rules/
// — no config wiring needed. We own the file entirely, so version tracking
// uses the top-of-file HTML comment marker (same as Aider/Cline).
//
// The type is named continueDev (not continue) because `continue` is a Go
// reserved word. Name() still returns the user-facing "continue" string.
type continueDev struct{}

func (continueDev) Name() string        { return "continue" }
func (continueDev) DisplayName() string { return "Continue.dev" }
func (continueDev) Scope() Scope        { return ScopeUser }

const (
	continueConfigDir = ".continue"
	continueRulesDir  = "rules"
	continueSkillFile = "deployhq.md"
)

func (c continueDev) configDir() (string, error) {
	home, err := homeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, continueConfigDir), nil
}

func (c continueDev) skillFile() (string, error) {
	cfg, err := c.configDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(cfg, continueRulesDir, continueSkillFile), nil
}

func (c continueDev) Detect() Status {
	cfg, err := c.configDir()
	if err != nil {
		return StatusNotInstalled
	}
	if _, err := os.Stat(cfg); err != nil {
		return StatusNotInstalled
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

func (c continueDev) Install() (string, error) {
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
