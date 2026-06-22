package skillinstaller

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/deployhq/deployhq-cli/skills"
)

func init() { Register(&copilot{}) }

// copilot installs the skill into GitHub Copilot's repo-level instructions.
//
// Unlike Claude/Cursor/Windsurf, Copilot has no reliable per-user "is it
// installed" signal — it could be a VS Code extension, a JetBrains plugin,
// `gh copilot`, or none of these. The thing Copilot consistently *does* read
// is `.github/copilot-instructions.md` in the current repo. So this target
// is project-scope: install writes to the cwd, not the user's home.
//
// Layout:
//   - <repo>/.github/copilot-instructions.md
//     Sentinel-bounded section that coexists with the user's own
//     instructions. Replaced in place on upgrade; user content outside
//     the markers is preserved.
//   - <repo>/.github/copilot/deployhq/SKILL.md + references/*.md
//     Full reference tree the agent can read on demand. Keeping the
//     instructions section terse and parking detail in a side directory
//     respects Copilot's instruction-length budget.
//
// Because installing modifies the user's repo, this target is excluded
// from the post-login 'dhq hello' prompt (see Scope == ScopeProject).
// Users opt in explicitly with 'dhq skills install --agent copilot'.
type copilot struct{}

func (copilot) Name() string        { return "copilot" }
func (copilot) DisplayName() string { return "GitHub Copilot" }
func (copilot) Scope() Scope        { return ScopeProject }

const (
	copilotInstructionsFile = ".github/copilot-instructions.md"
	copilotRefsDir          = ".github/copilot/deployhq"
)

func (c copilot) Detect() Status {
	root, ok := findRepoRoot()
	if !ok {
		// Not a git repo — nothing to install into. We treat this as
		// "not installed" so 'dhq skills list' stays informative without
		// implying we'd write to a random directory.
		return StatusNotInstalled
	}

	data, err := os.ReadFile(filepath.Join(root, copilotInstructionsFile))
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

func (c copilot) Install() (string, error) {
	root, ok := findRepoRoot()
	if !ok {
		cwd, _ := getCwd()
		return "", fmt.Errorf("not a git repository (cwd=%s); run from inside a repo or use a user-scope target", cwd)
	}

	// Refresh the reference tree at <repo>/.github/copilot/deployhq/.
	refsRoot := filepath.Join(root, copilotRefsDir)
	if err := os.RemoveAll(refsRoot); err != nil {
		return "", err
	}
	if err := writeEmbeddedTree(skills.FS, "deployhq", refsRoot); err != nil {
		return "", err
	}

	instrPath := filepath.Join(root, copilotInstructionsFile)
	if err := os.MkdirAll(filepath.Dir(instrPath), 0o755); err != nil {
		return "", err
	}
	existing, err := os.ReadFile(instrPath)
	if err != nil && !os.IsNotExist(err) {
		return "", err
	}

	section := buildCopilotSection(copilotRefsDir)
	merged := mergeSection(string(existing), section)
	if err := os.WriteFile(instrPath, []byte(merged), 0o644); err != nil {
		return "", err
	}
	return instrPath, nil
}

// buildCopilotSection produces the sentinel-bounded block we own in
// .github/copilot-instructions.md. Kept short because Copilot's instruction
// budget is finite and most projects have other guidance to share.
//
// The reference path is repo-relative so the rendered instructions remain
// portable when the repo is cloned to a different absolute path.
func buildCopilotSection(refsRelPath string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s%s%s\n", sectionBeginPrefix, skills.Version, sectionBeginSuffix)
	b.WriteString("This project uses DeployHQ. The `dhq` CLI is the canonical way to deploy ")
	b.WriteString("code, manage projects, servers, and repos via the DeployHQ platform.\n\n")
	b.WriteString("Skill guide and per-domain references (read these before suggesting `dhq` commands):\n")
	fmt.Fprintf(&b, "  - %s/SKILL.md\n", refsRelPath)
	fmt.Fprintf(&b, "  - %s/references/*.md\n\n", refsRelPath)
	b.WriteString("Domains: deployments, projects, servers, repos, configuration, operations, ")
	b.WriteString("global-resources, auth-setup. When suggesting deploys, prefer `dhq deploy` and ")
	b.WriteString("its flags over raw API calls.\n")
	b.WriteString(sectionEndMarker)
	return b.String()
}
