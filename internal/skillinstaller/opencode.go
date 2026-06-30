package skillinstaller

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/deployhq/deployhq-cli/skills"
)

func init() { Register(&opencode{}) }

// opencode installs the skill into OpenCode's user-level instructions file.
//
// Layout (same shape as Codex CLI, but XDG-aware):
//   - $XDG_CONFIG_HOME/opencode/AGENTS.md
//     (falls back to ~/.config/opencode/AGENTS.md when XDG_CONFIG_HOME is unset)
//   - <configdir>/opencode/deployhq-references/SKILL.md + references/*.md
type opencode struct{}

func (opencode) Name() string        { return "opencode" }
func (opencode) DisplayName() string { return "OpenCode" }
func (opencode) Scope() Scope        { return ScopeUser }

const (
	opencodeInstructionsFile = "AGENTS.md"
	opencodeRefsDir          = "deployhq-references"
)

// xdgConfigDir returns $XDG_CONFIG_HOME, or ~/.config as the XDG spec
// fallback. Lives at file scope so future XDG-aware targets can reuse it.
//
// The XDG Base Directory spec requires the variable to hold an absolute
// path; if it's set to a relative path, the spec says to ignore it and
// use the default. Honouring that here avoids accidentally writing into
// the dev's cwd when XDG_CONFIG_HOME is misconfigured.
func xdgConfigDir() (string, error) {
	if x := os.Getenv("XDG_CONFIG_HOME"); x != "" && filepath.IsAbs(x) {
		return x, nil
	}
	home, err := homeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config"), nil
}

func (o opencode) configDir() (string, error) {
	base, err := xdgConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "opencode"), nil
}

func (o opencode) Detect() Status {
	cfg, err := o.configDir()
	if err != nil {
		return StatusNotInstalled
	}
	if _, err := os.Stat(cfg); err != nil {
		return StatusNotInstalled
	}

	data, err := os.ReadFile(filepath.Join(cfg, opencodeInstructionsFile))
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

func (o opencode) Install() (string, error) {
	cfg, err := o.configDir()
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(cfg, 0o755); err != nil {
		return "", err
	}

	// writeEmbeddedTree replaces the tree atomically, so no pre-RemoveAll.
	refsRoot := filepath.Join(cfg, opencodeRefsDir)
	if err := writeEmbeddedTree(skills.FS, "deployhq", refsRoot); err != nil {
		return "", err
	}

	instrPath := filepath.Join(cfg, opencodeInstructionsFile)
	existing, err := os.ReadFile(instrPath)
	if err != nil && !os.IsNotExist(err) {
		return "", err
	}

	section := buildOpenCodeSection(refsRoot)
	merged := mergeSection(string(existing), section)
	if err := safeWriteFile(instrPath, []byte(merged), 0o644); err != nil {
		return "", err
	}
	return instrPath, nil
}

func buildOpenCodeSection(refsAbsPath string) string {
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
