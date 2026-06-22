package skillinstaller

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/deployhq/deployhq-cli/skills"
)

// withCwd swaps getCwd for the duration of the test, pointing at a temp dir
// that may or may not be a fake git repo depending on caller intent.
func withCwd(t *testing.T, dir string) {
	t.Helper()
	orig := getCwd
	getCwd = func() (string, error) { return dir, nil }
	t.Cleanup(func() { getCwd = orig })
}

// fakeRepo returns a temp dir with a .git subdirectory so copilot.inRepo()
// sees it as a real repo.
func fakeRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestCopilot_Detect_NotInRepo(t *testing.T) {
	withCwd(t, t.TempDir())
	if got := (copilot{}).Detect(); got != StatusNotInstalled {
		t.Fatalf("Detect() outside repo = %v, want StatusNotInstalled", got)
	}
}

func TestCopilot_Detect_InRepoNoInstructions(t *testing.T) {
	withCwd(t, fakeRepo(t))
	if got := (copilot{}).Detect(); got != StatusAvailable {
		t.Fatalf("Detect() in fresh repo = %v, want StatusAvailable", got)
	}
}

func TestCopilot_Detect_InRepoWithUnrelatedInstructions(t *testing.T) {
	dir := fakeRepo(t)
	withCwd(t, dir)
	if err := os.MkdirAll(filepath.Join(dir, ".github"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, copilotInstructionsFile), []byte("# repo guidance\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := (copilot{}).Detect(); got != StatusAvailable {
		t.Fatalf("Detect() with unrelated content = %v, want StatusAvailable", got)
	}
}

func TestCopilot_Install_OutsideRepo_Errors(t *testing.T) {
	withCwd(t, t.TempDir())
	_, err := (copilot{}).Install()
	if err == nil {
		t.Fatal("Install() outside repo expected to error")
	}
	if !strings.Contains(err.Error(), "not a git repository") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestCopilot_Install_WritesSectionAndRefs(t *testing.T) {
	dir := fakeRepo(t)
	withCwd(t, dir)

	got, err := (copilot{}).Install()
	if err != nil {
		t.Fatalf("Install() = %v", err)
	}
	want := filepath.Join(dir, copilotInstructionsFile)
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
		copilotRefsDir + "/SKILL.md",
	} {
		if !strings.Contains(string(body), must) {
			t.Errorf("instructions missing %q\n%s", must, body)
		}
	}

	if _, err := os.Stat(filepath.Join(dir, copilotRefsDir, "SKILL.md")); err != nil {
		t.Errorf("expected SKILL.md under refs root: %v", err)
	}
	refs, err := os.ReadDir(filepath.Join(dir, copilotRefsDir, "references"))
	if err != nil || len(refs) == 0 {
		t.Errorf("expected references/*.md: err=%v entries=%d", err, len(refs))
	}
}

func TestCopilot_Install_PreservesUserContent(t *testing.T) {
	dir := fakeRepo(t)
	withCwd(t, dir)
	if err := os.MkdirAll(filepath.Join(dir, ".github"), 0o755); err != nil {
		t.Fatal(err)
	}
	userRules := "# Team rules\n\nAlways write tests.\n"
	if err := os.WriteFile(filepath.Join(dir, copilotInstructionsFile), []byte(userRules), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := (copilot{}).Install(); err != nil {
		t.Fatalf("Install() = %v", err)
	}
	got, err := os.ReadFile(filepath.Join(dir, copilotInstructionsFile))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(got), "Always write tests.") {
		t.Errorf("user content lost; body=\n%s", got)
	}
	if !strings.Contains(string(got), sectionEndMarker) {
		t.Errorf("section not appended; body=\n%s", got)
	}
}

func TestCopilot_Install_Idempotent(t *testing.T) {
	dir := fakeRepo(t)
	withCwd(t, dir)

	if _, err := (copilot{}).Install(); err != nil {
		t.Fatal(err)
	}
	first, err := os.ReadFile(filepath.Join(dir, copilotInstructionsFile))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := (copilot{}).Install(); err != nil {
		t.Fatalf("second Install() = %v", err)
	}
	second, err := os.ReadFile(filepath.Join(dir, copilotInstructionsFile))
	if err != nil {
		t.Fatal(err)
	}
	if string(first) != string(second) {
		t.Errorf("idempotence broken\n--- first ---\n%s\n--- second ---\n%s", first, second)
	}
}

func TestCopilot_Detect_Outdated(t *testing.T) {
	dir := fakeRepo(t)
	withCwd(t, dir)
	if _, err := (copilot{}).Install(); err != nil {
		t.Fatal(err)
	}
	if got := (copilot{}).Detect(); got != StatusInstalled {
		t.Fatalf("post-install Detect() = %v, want StatusInstalled", got)
	}

	path := filepath.Join(dir, copilotInstructionsFile)
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	stale := strings.Replace(string(body), sectionBeginPrefix+skills.Version, sectionBeginPrefix+"0", 1)
	if err := os.WriteFile(path, []byte(stale), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := (copilot{}).Detect(); got != StatusOutdated {
		t.Fatalf("Detect() with stale marker = %v, want StatusOutdated", got)
	}
}

func TestCopilot_InRepo_FindsAncestor(t *testing.T) {
	// .git/ lives at root; cwd is a nested subdirectory.
	root := fakeRepo(t)
	sub := filepath.Join(root, "a", "b", "c")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	withCwd(t, sub)
	if got := (copilot{}).Detect(); got != StatusAvailable {
		t.Fatalf("Detect() nested in repo = %v, want StatusAvailable", got)
	}
}

func TestCopilot_Scope_IsProject(t *testing.T) {
	if got := (copilot{}).Scope(); got != ScopeProject {
		t.Errorf("Scope() = %v, want ScopeProject", got)
	}
}

func TestRegistry_ContainsCopilot(t *testing.T) {
	if Find("copilot") == nil {
		t.Fatal("copilot target not registered")
	}
}
