package skillinstaller

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/deployhq/deployhq-cli/skills"
)

func init() { Register(&cursor{}) }

// cursor installs the skill into Cursor's user-level rules directory.
//
// Layout: ~/.cursor/rules/deployhq.mdc (single file — Cursor only loads
//         *.mdc and has no directory tree concept like Claude's skills/)
// Version marker: ~/.cursor/rules/.deployhq-skill-version (hidden, so
//         Cursor's *.mdc glob ignores it)
//
// The .mdc has Cursor frontmatter (description, alwaysApply) and the body
// concatenates SKILL.md with every reference file under clear "## reference:"
// headers so the agent can find them in one read.
type cursor struct{}

func (cursor) Name() string        { return "cursor" }
func (cursor) DisplayName() string { return "Cursor" }
func (cursor) Scope() Scope        { return ScopeUser }

const cursorSkillFile = "deployhq.mdc"

func (c cursor) configDir() (string, error) {
	home, err := homeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".cursor"), nil
}

func (c cursor) rulesDir() (string, error) {
	cfg, err := c.configDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(cfg, "rules"), nil
}

func (c cursor) Detect() Status {
	cfg, err := c.configDir()
	if err != nil {
		return StatusNotInstalled
	}
	if _, err := os.Stat(cfg); err != nil {
		// No ~/.cursor → Cursor has never run on this machine.
		return StatusNotInstalled
	}

	rules, err := c.rulesDir()
	if err != nil {
		return StatusNotInstalled
	}
	if _, err := os.Stat(filepath.Join(rules, cursorSkillFile)); err != nil {
		return StatusAvailable
	}

	switch readVersion(filepath.Join(rules, versionMarker)) {
	case skills.Version:
		return StatusInstalled
	default:
		return StatusOutdated
	}
}

func (c cursor) Install() (string, error) {
	rules, err := c.rulesDir()
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(rules, 0o755); err != nil {
		return "", err
	}

	content, err := buildCursorMDC(skills.FS, "deployhq")
	if err != nil {
		return "", err
	}

	dst := filepath.Join(rules, cursorSkillFile)
	if err := os.WriteFile(dst, content, 0o644); err != nil {
		return "", err
	}
	if err := os.WriteFile(filepath.Join(rules, versionMarker), []byte(skills.Version+"\n"), 0o644); err != nil {
		return "", err
	}
	return dst, nil
}

// buildCursorMDC wraps the flattened skill body in Cursor's .mdc frontmatter.
//
// Output shape:
//
//	---
//	description: <extracted from SKILL.md frontmatter, collapsed to one line>
//	alwaysApply: false
//	---
//
//	<SKILL.md body + concatenated references>
//
// alwaysApply: false means Cursor treats this as an agent-requested rule —
// the agent decides when to pull it in based on the description. Same model
// as Claude Code's progressive-disclosure skill discovery.
func buildCursorMDC(efs fs.FS, root string) ([]byte, error) {
	description, body, err := flattenSkill(efs, root)
	if err != nil {
		return nil, err
	}
	if description == "" {
		description = "DeployHQ CLI — deploy code, manage servers, automate infrastructure via the dhq command."
	}

	var buf strings.Builder
	buf.WriteString("---\n")
	fmt.Fprintf(&buf, "description: %s\n", oneLine(description))
	buf.WriteString("alwaysApply: false\n")
	buf.WriteString("---\n\n")
	buf.Write(body)
	return []byte(buf.String()), nil
}
