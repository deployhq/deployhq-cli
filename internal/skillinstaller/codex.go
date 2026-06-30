package skillinstaller

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/deployhq/deployhq-cli/skills"
)

func init() { Register(&codex{}) }

// codex installs the skill into OpenAI's Codex CLI user-level instructions.
//
// Layout:
//   - ~/.codex/AGENTS.md
//     A single file Codex CLI reads for global agent instructions. We own
//     only a sentinel-bounded section in it; user content is preserved
//     byte-for-byte across (re)installs.
//   - ~/.codex/deployhq-references/SKILL.md + references/*.md
//     The full reference tree, written so the agent can pull in detail on
//     demand. The instructions section in AGENTS.md is intentionally short
//     and points at this tree by absolute path.
//
// Same shape as the Windsurf target — single shared instructions file with
// section-level coexistence. The only differences are the paths and the
// per-target intro text.
type codex struct{}

func (codex) Name() string        { return "codex" }
func (codex) DisplayName() string { return "Codex CLI" }
func (codex) Scope() Scope        { return ScopeUser }

const (
	codexAgentsFile = "AGENTS.md"
	codexRefsDir    = "deployhq-references"
)

func (c codex) configDir() (string, error) {
	home, err := homeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".codex"), nil
}

func (c codex) Detect() Status {
	cfg, err := c.configDir()
	if err != nil {
		return StatusNotInstalled
	}
	if _, err := os.Stat(cfg); err != nil {
		// No ~/.codex → Codex CLI has never run on this machine.
		return StatusNotInstalled
	}

	data, err := os.ReadFile(filepath.Join(cfg, codexAgentsFile))
	if err != nil {
		if os.IsNotExist(err) {
			// Codex installed but AGENTS.md not written yet — skill available.
			return StatusAvailable
		}
		// Permission/IO error — can't determine state; treat as not installed.
		return StatusNotInstalled
	}
	switch parseSectionVersion(string(data)) {
	case "":
		return StatusAvailable
	case skills.Version:
		return StatusInstalled
	default:
		return StatusOutdated
	}
}

func (c codex) Install() (string, error) {
	cfg, err := c.configDir()
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(cfg, 0o755); err != nil {
		return "", err
	}

	// Refresh the reference tree at ~/.codex/deployhq-references/.
	// writeEmbeddedTree replaces it atomically, so no pre-RemoveAll.
	refsRoot := filepath.Join(cfg, codexRefsDir)
	if err := writeEmbeddedTree(skills.FS, "deployhq", refsRoot); err != nil {
		return "", err
	}

	agentsPath := filepath.Join(cfg, codexAgentsFile)
	existing, err := os.ReadFile(agentsPath)
	if err != nil && !os.IsNotExist(err) {
		return "", err
	}

	section := buildCodexSection(refsRoot)
	merged := mergeSection(string(existing), section)
	if err := safeWriteFile(agentsPath, []byte(merged), 0o644); err != nil {
		return "", err
	}
	return agentsPath, nil
}

// buildCodexSection produces the sentinel-bounded block we own in
// ~/.codex/AGENTS.md. The reference path is absolute because Codex CLI
// reads AGENTS.md verbatim — agents won't infer the user's home directory.
func buildCodexSection(refsAbsPath string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s%s%s\n", sectionBeginPrefix, skills.Version, sectionBeginSuffix)
	b.WriteString("DeployHQ CLI (`dhq`) is available. Use it to deploy code, manage projects, ")
	b.WriteString("servers, and repos via the DeployHQ platform.\n\n")
	b.WriteString("Skill guide and per-domain references (read before suggesting `dhq` commands):\n")
	fmt.Fprintf(&b, "  - %s/SKILL.md\n", refsAbsPath)
	fmt.Fprintf(&b, "  - %s/references/*.md\n\n", refsAbsPath)
	b.WriteString("Domains: deployments, projects, servers, repos, configuration, operations, ")
	b.WriteString("global-resources, auth-setup. When the user asks to deploy, prefer `dhq deploy` ")
	b.WriteString("with its flags over raw API calls.\n")
	b.WriteString(sectionEndMarker)
	return b.String()
}
