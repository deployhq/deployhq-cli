package skillinstaller

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/deployhq/deployhq-cli/skills"
)

func TestAntigravity_Detect_NotInRepo(t *testing.T) {
	withCwd(t, t.TempDir())
	if got := (antigravity{}).Detect(); got != StatusNotInstalled {
		t.Fatalf("Detect() outside repo = %v, want StatusNotInstalled", got)
	}
}

func TestAntigravity_Detect_InRepoNoAgentsFile(t *testing.T) {
	withCwd(t, fakeRepo(t))
	if got := (antigravity{}).Detect(); got != StatusAvailable {
		t.Fatalf("Detect() = %v, want StatusAvailable", got)
	}
}

func TestAntigravity_Install_OutsideRepo_Errors(t *testing.T) {
	withCwd(t, t.TempDir())
	if _, err := (antigravity{}).Install(); err == nil {
		t.Fatal("expected error outside repo")
	}
}

func TestAntigravity_Install_WritesSectionAndRefs(t *testing.T) {
	dir := fakeRepo(t)
	withCwd(t, dir)

	got, err := (antigravity{}).Install()
	if err != nil {
		t.Fatalf("Install() = %v", err)
	}
	want := filepath.Join(dir, antigravityInstructionsFile)
	if got != want {
		t.Errorf("Install() path = %q, want %q", got, want)
	}

	body, err := os.ReadFile(want)
	if err != nil {
		t.Fatal(err)
	}
	for _, must := range []string{
		sectionBeginPrefix + skills.Version + sectionBeginSuffix,
		sectionEndMarker,
		antigravityRefsDir + "/SKILL.md",
	} {
		if !strings.Contains(string(body), must) {
			t.Errorf("AGENTS.md missing %q\n%s", must, body)
		}
	}

	if _, err := os.Stat(filepath.Join(dir, antigravityRefsDir, "SKILL.md")); err != nil {
		t.Errorf("expected SKILL.md under refs root: %v", err)
	}
	refs, err := os.ReadDir(filepath.Join(dir, antigravityRefsDir, "references"))
	if err != nil || len(refs) == 0 {
		t.Errorf("expected references/*.md: err=%v entries=%d", err, len(refs))
	}
}

func TestAntigravity_Install_PreservesCrossToolContent(t *testing.T) {
	// AGENTS.md is a shared convention — the user may already have
	// instructions for other tools in there. Those must survive install.
	dir := fakeRepo(t)
	withCwd(t, dir)
	userContent := "# Instructions for All Agents\n\nAlways write tests.\n"
	if err := os.WriteFile(filepath.Join(dir, antigravityInstructionsFile), []byte(userContent), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := (antigravity{}).Install(); err != nil {
		t.Fatalf("Install() = %v", err)
	}
	got, err := os.ReadFile(filepath.Join(dir, antigravityInstructionsFile))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(got), "Always write tests.") {
		t.Errorf("cross-tool content lost:\n%s", got)
	}
	if !strings.Contains(string(got), sectionEndMarker) {
		t.Errorf("DeployHQ section not appended:\n%s", got)
	}
}

func TestAntigravity_Install_Idempotent(t *testing.T) {
	dir := fakeRepo(t)
	withCwd(t, dir)

	if _, err := (antigravity{}).Install(); err != nil {
		t.Fatal(err)
	}
	first, err := os.ReadFile(filepath.Join(dir, antigravityInstructionsFile))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := (antigravity{}).Install(); err != nil {
		t.Fatalf("second Install() = %v", err)
	}
	second, err := os.ReadFile(filepath.Join(dir, antigravityInstructionsFile))
	if err != nil {
		t.Fatal(err)
	}
	if string(first) != string(second) {
		t.Errorf("idempotence broken\n--- first ---\n%s\n--- second ---\n%s", first, second)
	}
}

func TestAntigravity_Detect_Outdated(t *testing.T) {
	dir := fakeRepo(t)
	withCwd(t, dir)
	if _, err := (antigravity{}).Install(); err != nil {
		t.Fatal(err)
	}
	if got := (antigravity{}).Detect(); got != StatusInstalled {
		t.Fatalf("post-install Detect() = %v, want StatusInstalled", got)
	}

	path := filepath.Join(dir, antigravityInstructionsFile)
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	stale := strings.Replace(string(body), sectionBeginPrefix+skills.Version, sectionBeginPrefix+"0", 1)
	if err := os.WriteFile(path, []byte(stale), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := (antigravity{}).Detect(); got != StatusOutdated {
		t.Fatalf("Detect() stale marker = %v, want StatusOutdated", got)
	}
}

func TestAntigravity_Scope_IsProject(t *testing.T) {
	if got := (antigravity{}).Scope(); got != ScopeProject {
		t.Errorf("Scope() = %v, want ScopeProject", got)
	}
}

func TestRegistry_ContainsAntigravity(t *testing.T) {
	if Find("antigravity") == nil {
		t.Fatal("antigravity target not registered")
	}
}
