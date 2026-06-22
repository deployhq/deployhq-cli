package skillinstaller

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFindRepoRoot_WalksAncestors(t *testing.T) {
	root := fakeRepo(t)
	sub := filepath.Join(root, "a", "b", "c")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	withCwd(t, sub)

	got, ok := findRepoRoot()
	if !ok {
		t.Fatal("findRepoRoot() returned !ok inside a repo subdirectory")
	}
	if got != root {
		t.Errorf("findRepoRoot() = %q, want %q", got, root)
	}
}

func TestFindRepoRoot_OutsideRepo(t *testing.T) {
	withCwd(t, t.TempDir())
	if _, ok := findRepoRoot(); ok {
		t.Fatal("findRepoRoot() returned ok outside a git repo")
	}
}

// Regression: each project-scope target must write at the repo root, not
// the cwd, when invoked from a subdirectory. Bug surfaced by Codex review
// on copilot.go; the same pattern existed in cline/kiro/antigravity.

func TestCopilot_Install_WritesAtRepoRoot_FromSubdirectory(t *testing.T) {
	root := fakeRepo(t)
	sub := filepath.Join(root, "deep", "sub")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	withCwd(t, sub)

	got, err := (copilot{}).Install()
	if err != nil {
		t.Fatalf("Install() = %v", err)
	}
	want := filepath.Join(root, copilotInstructionsFile)
	if got != want {
		t.Errorf("Install() path = %q, want %q (must be at repo root, not subdir)", got, want)
	}
	if _, err := os.Stat(filepath.Join(sub, copilotInstructionsFile)); err == nil {
		t.Errorf("Install() wrote to subdir %s — must be at repo root", sub)
	}
}

func TestCline_Install_WritesAtRepoRoot_FromSubdirectory(t *testing.T) {
	root := fakeRepo(t)
	sub := filepath.Join(root, "deep", "sub")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	withCwd(t, sub)

	got, err := (cline{}).Install()
	if err != nil {
		t.Fatalf("Install() = %v", err)
	}
	want := filepath.Join(root, clineRulesDir, clineSkillFile)
	if got != want {
		t.Errorf("Install() path = %q, want %q (must be at repo root)", got, want)
	}
	if _, err := os.Stat(filepath.Join(sub, clineRulesDir)); err == nil {
		t.Errorf("Install() created %s/.clinerules — must be at repo root", sub)
	}
}

func TestKiro_Install_WritesAtRepoRoot_FromSubdirectory(t *testing.T) {
	root := fakeRepo(t)
	sub := filepath.Join(root, "deep", "sub")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	withCwd(t, sub)

	got, err := (kiro{}).Install()
	if err != nil {
		t.Fatalf("Install() = %v", err)
	}
	want := filepath.Join(root, kiroSteeringDir, kiroSkillFile)
	if got != want {
		t.Errorf("Install() path = %q, want %q (must be at repo root)", got, want)
	}
}

func TestAntigravity_Install_WritesAtRepoRoot_FromSubdirectory(t *testing.T) {
	root := fakeRepo(t)
	sub := filepath.Join(root, "deep", "sub")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	withCwd(t, sub)

	got, err := (antigravity{}).Install()
	if err != nil {
		t.Fatalf("Install() = %v", err)
	}
	want := filepath.Join(root, antigravityInstructionsFile)
	if got != want {
		t.Errorf("Install() path = %q, want %q (must be at repo root)", got, want)
	}
}
