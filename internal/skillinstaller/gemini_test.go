package skillinstaller

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/deployhq/deployhq-cli/skills"
)

func TestGemini_Detect_NoConfigDir(t *testing.T) {
	withHomeDir(t, t.TempDir())
	if got := (gemini{}).Detect(); got != StatusNotInstalled {
		t.Fatalf("Detect() = %v, want StatusNotInstalled", got)
	}
}

func TestGemini_Detect_InstalledNoInstructions(t *testing.T) {
	home := t.TempDir()
	withHomeDir(t, home)
	if err := os.MkdirAll(filepath.Join(home, ".gemini"), 0o755); err != nil {
		t.Fatal(err)
	}
	if got := (gemini{}).Detect(); got != StatusAvailable {
		t.Fatalf("Detect() = %v, want StatusAvailable", got)
	}
}

func TestGemini_Install_WritesSectionAndRefs(t *testing.T) {
	home := t.TempDir()
	withHomeDir(t, home)
	if err := os.MkdirAll(filepath.Join(home, ".gemini"), 0o755); err != nil {
		t.Fatal(err)
	}

	got, err := (gemini{}).Install()
	if err != nil {
		t.Fatalf("Install() = %v", err)
	}
	want := filepath.Join(home, ".gemini", geminiInstructionsFile)
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
		filepath.Join(home, ".gemini", geminiRefsDir, "SKILL.md"),
	} {
		if !strings.Contains(string(body), must) {
			t.Errorf("GEMINI.md missing %q\n%s", must, body)
		}
	}

	if _, err := os.Stat(filepath.Join(home, ".gemini", geminiRefsDir, "SKILL.md")); err != nil {
		t.Errorf("expected SKILL.md under refs root: %v", err)
	}
	refs, err := os.ReadDir(filepath.Join(home, ".gemini", geminiRefsDir, "references"))
	if err != nil || len(refs) == 0 {
		t.Errorf("expected references/*.md: err=%v entries=%d", err, len(refs))
	}
}

func TestGemini_Install_PreservesUserContent(t *testing.T) {
	home := t.TempDir()
	withHomeDir(t, home)
	cfg := filepath.Join(home, ".gemini")
	if err := os.MkdirAll(cfg, 0o755); err != nil {
		t.Fatal(err)
	}
	userContent := "# My Gemini rules\n\nBe concise.\n"
	if err := os.WriteFile(filepath.Join(cfg, geminiInstructionsFile), []byte(userContent), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := (gemini{}).Install(); err != nil {
		t.Fatalf("Install() = %v", err)
	}
	got, err := os.ReadFile(filepath.Join(cfg, geminiInstructionsFile))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(got), "Be concise.") {
		t.Errorf("user content lost:\n%s", got)
	}
	if !strings.Contains(string(got), sectionEndMarker) {
		t.Errorf("section not appended:\n%s", got)
	}
}

func TestGemini_Install_Idempotent(t *testing.T) {
	home := t.TempDir()
	withHomeDir(t, home)
	if err := os.MkdirAll(filepath.Join(home, ".gemini"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := (gemini{}).Install(); err != nil {
		t.Fatal(err)
	}
	first, err := os.ReadFile(filepath.Join(home, ".gemini", geminiInstructionsFile))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := (gemini{}).Install(); err != nil {
		t.Fatalf("second Install() = %v", err)
	}
	second, err := os.ReadFile(filepath.Join(home, ".gemini", geminiInstructionsFile))
	if err != nil {
		t.Fatal(err)
	}
	if string(first) != string(second) {
		t.Errorf("idempotence broken\n--- first ---\n%s\n--- second ---\n%s", first, second)
	}
}

func TestGemini_Detect_Outdated(t *testing.T) {
	home := t.TempDir()
	withHomeDir(t, home)
	if err := os.MkdirAll(filepath.Join(home, ".gemini"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := (gemini{}).Install(); err != nil {
		t.Fatal(err)
	}
	if got := (gemini{}).Detect(); got != StatusInstalled {
		t.Fatalf("post-install Detect() = %v, want StatusInstalled", got)
	}

	path := filepath.Join(home, ".gemini", geminiInstructionsFile)
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	stale := strings.Replace(string(body), sectionBeginPrefix+skills.Version, sectionBeginPrefix+"0", 1)
	if err := os.WriteFile(path, []byte(stale), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := (gemini{}).Detect(); got != StatusOutdated {
		t.Fatalf("Detect() stale marker = %v, want StatusOutdated", got)
	}
}

func TestGemini_Scope_IsUser(t *testing.T) {
	if got := (gemini{}).Scope(); got != ScopeUser {
		t.Errorf("Scope() = %v, want ScopeUser", got)
	}
}

func TestRegistry_ContainsGemini(t *testing.T) {
	if Find("gemini") == nil {
		t.Fatal("gemini target not registered")
	}
}
