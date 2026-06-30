package skillinstaller

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/deployhq/deployhq-cli/skills"
)

func init() { Register(&gemini{}) }

// gemini installs the skill into Google's Gemini CLI user-level
// instructions file.
//
// Layout (same shape as Codex CLI):
//   - ~/.gemini/GEMINI.md
//     Single shared file; we own a sentinel-bounded section, user
//     content outside it is preserved.
//   - ~/.gemini/deployhq-references/SKILL.md + references/*.md
//     Full reference tree the agent reads on demand.
type gemini struct{}

func (gemini) Name() string        { return "gemini" }
func (gemini) DisplayName() string { return "Gemini CLI" }
func (gemini) Scope() Scope        { return ScopeUser }

const (
	geminiInstructionsFile = "GEMINI.md"
	geminiRefsDir          = "deployhq-references"
)

func (g gemini) configDir() (string, error) {
	home, err := homeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".gemini"), nil
}

func (g gemini) Detect() Status {
	cfg, err := g.configDir()
	if err != nil {
		return StatusNotInstalled
	}
	if _, err := os.Stat(cfg); err != nil {
		return StatusNotInstalled
	}

	data, err := os.ReadFile(filepath.Join(cfg, geminiInstructionsFile))
	if err != nil {
		if os.IsNotExist(err) {
			return StatusAvailable
		}
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

func (g gemini) Install() (string, error) {
	cfg, err := g.configDir()
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(cfg, 0o755); err != nil {
		return "", err
	}

	// writeEmbeddedTree replaces the tree atomically, so no pre-RemoveAll.
	refsRoot := filepath.Join(cfg, geminiRefsDir)
	if err := writeEmbeddedTree(skills.FS, "deployhq", refsRoot); err != nil {
		return "", err
	}

	instrPath := filepath.Join(cfg, geminiInstructionsFile)
	existing, err := os.ReadFile(instrPath)
	if err != nil && !os.IsNotExist(err) {
		return "", err
	}

	section := buildGeminiSection(refsRoot)
	merged := mergeSection(string(existing), section)
	if err := safeWriteFile(instrPath, []byte(merged), 0o644); err != nil {
		return "", err
	}
	return instrPath, nil
}

func buildGeminiSection(refsAbsPath string) string {
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
