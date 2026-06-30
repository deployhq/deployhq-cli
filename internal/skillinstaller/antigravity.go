package skillinstaller

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/deployhq/deployhq-cli/skills"
)

func init() { Register(&antigravity{}) }

// antigravity installs the skill into the repo-root AGENTS.md file that
// Google's Antigravity IDE reads for project context.
//
// AGENTS.md is a cross-tool convention (Codex CLI inside projects, Amp,
// and a handful of others read it too), so we own only a sentinel-bounded
// section. User content — including instructions for other agents — is
// preserved byte-for-byte.
//
// Layout:
//   - <repo>/AGENTS.md
//     Sentinel-bounded section.
//   - <repo>/.antigravity/deployhq/SKILL.md + references/*.md
//     Full reference tree, namespaced under .antigravity/ to keep the
//     repo root tidy. The section in AGENTS.md points at this tree by
//     repo-relative path so it stays portable across clones.
//
// ScopeProject — modifies the user's repo, opt-in via:
//
//	dhq skills install --agent antigravity
type antigravity struct{}

func (antigravity) Name() string        { return "antigravity" }
func (antigravity) DisplayName() string { return "Antigravity" }
func (antigravity) Scope() Scope        { return ScopeProject }

const (
	antigravityInstructionsFile = "AGENTS.md"
	antigravityRefsDir          = ".antigravity/deployhq"
)

func (a antigravity) Detect() Status {
	root, ok := findRepoRoot()
	if !ok {
		return StatusNotInstalled
	}
	data, err := os.ReadFile(filepath.Join(root, antigravityInstructionsFile))
	if err != nil {
		return StatusAvailable
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

func (a antigravity) Install() (string, error) {
	root, ok := findRepoRoot()
	if !ok {
		cwd, _ := getCwd()
		return "", fmt.Errorf("not a git repository (cwd=%s); run from inside a repo", cwd)
	}

	// Refresh the reference tree at <repo>/.antigravity/deployhq/.
	// writeEmbeddedTree replaces it atomically, so no pre-RemoveAll.
	refsRoot := filepath.Join(root, antigravityRefsDir)
	if err := writeEmbeddedTree(skills.FS, "deployhq", refsRoot); err != nil {
		return "", err
	}

	instrPath := filepath.Join(root, antigravityInstructionsFile)
	existing, err := os.ReadFile(instrPath)
	if err != nil && !os.IsNotExist(err) {
		return "", err
	}

	section := buildAntigravitySection(antigravityRefsDir)
	merged := mergeSection(string(existing), section)
	if err := safeWriteFile(instrPath, []byte(merged), 0o644); err != nil {
		return "", err
	}
	return instrPath, nil
}

// buildAntigravitySection produces the sentinel-bounded block in AGENTS.md.
// Repo-relative reference paths keep the rendered file portable across
// clones — agents resolve them from the repo root, not from where dhq
// happened to run.
func buildAntigravitySection(refsRelPath string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s%s%s\n", sectionBeginPrefix, skills.Version, sectionBeginSuffix)
	b.WriteString("This project uses DeployHQ. The `dhq` CLI is the canonical way to deploy ")
	b.WriteString("code, manage projects, servers, and repos via the DeployHQ platform.\n\n")
	b.WriteString("Skill guide and per-domain references (read before suggesting `dhq` commands):\n")
	fmt.Fprintf(&b, "  - %s/SKILL.md\n", refsRelPath)
	fmt.Fprintf(&b, "  - %s/references/*.md\n\n", refsRelPath)
	b.WriteString("Domains: deployments, projects, servers, repos, configuration, operations, ")
	b.WriteString("global-resources, auth-setup. When suggesting deploys, prefer `dhq deploy` and ")
	b.WriteString("its flags over raw API calls.\n")
	b.WriteString(sectionEndMarker)
	return b.String()
}
