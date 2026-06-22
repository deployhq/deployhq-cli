package skillinstaller

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/deployhq/deployhq-cli/skills"
)

func init() { Register(&kiro{}) }

// kiro installs the skill as an AWS Kiro CLI steering file.
//
// Kiro discovers per-project context via "steering" files — every *.md
// under <repo>/.kiro/steering/ is loaded into the agent's context. We
// own a single file in that directory; the user's other steering files
// are untouched.
//
// Layout: <repo>/.kiro/steering/deployhq.md
//
// ScopeProject — modifying the user's repo as a side effect of `dhq hello`
// would be hostile. Opt in via:
//
//	dhq skills install --agent kiro
type kiro struct{}

func (kiro) Name() string        { return "kiro" }
func (kiro) DisplayName() string { return "Kiro CLI" }
func (kiro) Scope() Scope        { return ScopeProject }

const (
	kiroSteeringDir = ".kiro/steering"
	kiroSkillFile   = "deployhq.md"
)

func (k kiro) Detect() Status {
	root, ok := findRepoRoot()
	if !ok {
		return StatusNotInstalled
	}
	data, err := os.ReadFile(filepath.Join(root, kiroSteeringDir, kiroSkillFile))
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

func (k kiro) Install() (string, error) {
	root, ok := findRepoRoot()
	if !ok {
		cwd, _ := getCwd()
		return "", fmt.Errorf("not a git repository (cwd=%s); run from inside a repo", cwd)
	}
	p := filepath.Join(root, kiroSteeringDir, kiroSkillFile)
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
