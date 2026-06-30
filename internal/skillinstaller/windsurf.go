package skillinstaller

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/deployhq/deployhq-cli/skills"
)

func init() { Register(&windsurf{}) }

// windsurf installs the skill into Windsurf's global rules file.
//
// Layout:
//   - ~/.codeium/windsurf/memories/global_rules.md
//     A single file shared with the user's own rules. We own only a
//     sentinel-bounded section; everything outside it is preserved
//     verbatim across (re)installs.
//   - ~/.codeium/windsurf/memories/deployhq-references/*.md
//     The full reference tree, written so the agent can read deep docs on
//     demand. global_rules.md has a length budget historically — keeping
//     the rules section terse and parking detail in a side directory keeps
//     it inside that budget.
//
// Note: Windows users keep Codeium data under %APPDATA%\codeium, not
// %USERPROFILE%\.codeium. Until we add Windows-specific path handling,
// Detect() will return StatusNotInstalled on Windows even when Windsurf is
// installed — that's the correct safe default (no false-positive prompts).
type windsurf struct{}

func (windsurf) Name() string        { return "windsurf" }
func (windsurf) DisplayName() string { return "Windsurf" }
func (windsurf) Scope() Scope        { return ScopeUser }

const (
	windsurfRulesFile = "global_rules.md"
	windsurfRefsDir   = "deployhq-references"
)

func (w windsurf) memoriesDir() (string, error) {
	home, err := homeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".codeium", "windsurf", "memories"), nil
}

func (w windsurf) installedAtDir() (string, error) {
	home, err := homeDir()
	if err != nil {
		return "", err
	}
	// We treat ~/.codeium/windsurf as the "Windsurf is installed" signal.
	return filepath.Join(home, ".codeium", "windsurf"), nil
}

func (w windsurf) Detect() Status {
	dir, err := w.installedAtDir()
	if err != nil {
		return StatusNotInstalled
	}
	if _, err := os.Stat(dir); err != nil {
		return StatusNotInstalled
	}

	mem, err := w.memoriesDir()
	if err != nil {
		return StatusNotInstalled
	}
	rulesPath := filepath.Join(mem, windsurfRulesFile)
	data, err := os.ReadFile(rulesPath)
	if err != nil {
		if os.IsNotExist(err) {
			// Windsurf installed but no rules file yet — skill is available.
			return StatusAvailable
		}
		// Permission/IO error — can't determine state; treat as not installed.
		return StatusNotInstalled
	}

	version := parseSectionVersion(string(data))
	switch version {
	case "":
		return StatusAvailable
	case skills.Version:
		return StatusInstalled
	default:
		return StatusOutdated
	}
}

func (w windsurf) Install() (string, error) {
	mem, err := w.memoriesDir()
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(mem, 0o755); err != nil {
		return "", err
	}

	// Write the deep-reference tree alongside global_rules.md so the agent
	// can pull in detail on demand. SKILL.md is intentionally included
	// here too — the rules-section pointer can address it by relative path.
	refsRoot := filepath.Join(mem, windsurfRefsDir)
	if err := os.RemoveAll(refsRoot); err != nil {
		return "", err
	}
	if err := writeEmbeddedTree(skills.FS, "deployhq", refsRoot); err != nil {
		return "", err
	}

	rulesPath := filepath.Join(mem, windsurfRulesFile)
	existing, err := os.ReadFile(rulesPath)
	if err != nil && !os.IsNotExist(err) {
		return "", err
	}

	section := buildWindsurfSection(refsRoot)
	merged := mergeSection(string(existing), section)

	if err := safeWriteFile(rulesPath, []byte(merged), 0o644); err != nil {
		return "", err
	}
	return rulesPath, nil
}

// buildWindsurfSection produces the sentinel-bounded block we own in
// global_rules.md. It is intentionally short — Windsurf's global rules
// budget is tight, so we keep this to a pointer + key behaviours and rely
// on the agent reading deployhq-references/ for detail.
func buildWindsurfSection(refsAbsPath string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s%s%s\n", sectionBeginPrefix, skills.Version, sectionBeginSuffix)
	b.WriteString("DeployHQ CLI (`dhq`) is available. Use it to deploy code, manage projects, ")
	b.WriteString("servers, and repos via the DeployHQ platform.\n\n")
	b.WriteString("Full skill guide and per-domain references live at:\n")
	fmt.Fprintf(&b, "  %s/SKILL.md\n", refsAbsPath)
	fmt.Fprintf(&b, "  %s/references/*.md\n\n", refsAbsPath)
	b.WriteString("When the user wants to deploy, check deployment status, manage projects/")
	b.WriteString("servers, or interact with the DeployHQ platform, read SKILL.md first, then ")
	b.WriteString("the relevant reference file for the domain (deployments, projects, servers, ")
	b.WriteString("repos, configuration, operations, global-resources, auth-setup).\n")
	b.WriteString(sectionEndMarker)
	return b.String()
}

// joinAround stitches pre + section + post with a single blank line between
// non-empty halves, and exactly one trailing newline. Shared with copilot
// and codex via mergeSection in section.go.
func joinAround(pre, section, post string) string {
	var b strings.Builder
	if pre != "" {
		b.WriteString(pre)
		b.WriteString("\n\n")
	}
	b.WriteString(section)
	if post != "" {
		b.WriteString("\n\n")
		b.WriteString(strings.TrimRight(post, "\n"))
	}
	b.WriteString("\n")
	return b.String()
}
