package skillinstaller

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/deployhq/deployhq-cli/skills"
)

func TestWindsurf_Detect_NoConfigDir(t *testing.T) {
	withHomeDir(t, t.TempDir())
	if got := (windsurf{}).Detect(); got != StatusNotInstalled {
		t.Fatalf("Detect() = %v, want StatusNotInstalled", got)
	}
}

func TestWindsurf_Detect_InstalledNoRulesFile(t *testing.T) {
	home := t.TempDir()
	withHomeDir(t, home)
	if err := os.MkdirAll(filepath.Join(home, ".codeium", "windsurf"), 0o755); err != nil {
		t.Fatal(err)
	}
	if got := (windsurf{}).Detect(); got != StatusAvailable {
		t.Fatalf("Detect() = %v, want StatusAvailable", got)
	}
}

func TestWindsurf_Detect_RulesFileWithoutSection(t *testing.T) {
	home := t.TempDir()
	withHomeDir(t, home)
	memDir := filepath.Join(home, ".codeium", "windsurf", "memories")
	if err := os.MkdirAll(memDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(memDir, windsurfRulesFile), []byte("# My user rules\n\nBe concise.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := (windsurf{}).Detect(); got != StatusAvailable {
		t.Fatalf("Detect() = %v, want StatusAvailable", got)
	}
}

func TestWindsurf_Install_WritesSectionAndRefs(t *testing.T) {
	home := t.TempDir()
	withHomeDir(t, home)
	if err := os.MkdirAll(filepath.Join(home, ".codeium", "windsurf"), 0o755); err != nil {
		t.Fatal(err)
	}

	got, err := (windsurf{}).Install()
	if err != nil {
		t.Fatalf("Install() error = %v", err)
	}
	want := filepath.Join(home, ".codeium", "windsurf", "memories", windsurfRulesFile)
	if got != want {
		t.Errorf("Install() path = %q, want %q", got, want)
	}

	rules, err := os.ReadFile(want)
	if err != nil {
		t.Fatalf("read rules: %v", err)
	}
	body := string(rules)
	if !strings.Contains(body, sectionBeginPrefix+skills.Version+sectionBeginSuffix) {
		t.Errorf("rules missing BEGIN marker with current version; body=%q", body)
	}
	if !strings.Contains(body, sectionEndMarker) {
		t.Errorf("rules missing END marker; body=%q", body)
	}

	// References tree should land alongside.
	refsRoot := filepath.Join(home, ".codeium", "windsurf", "memories", windsurfRefsDir)
	if _, err := os.Stat(filepath.Join(refsRoot, "SKILL.md")); err != nil {
		t.Errorf("expected SKILL.md under refs root: %v", err)
	}
	entries, err := os.ReadDir(filepath.Join(refsRoot, "references"))
	if err != nil || len(entries) == 0 {
		t.Errorf("expected references/*.md under refs root: err=%v entries=%d", err, len(entries))
	}
}

func TestWindsurf_Install_PreservesUserRules(t *testing.T) {
	home := t.TempDir()
	withHomeDir(t, home)
	memDir := filepath.Join(home, ".codeium", "windsurf", "memories")
	if err := os.MkdirAll(memDir, 0o755); err != nil {
		t.Fatal(err)
	}
	userRules := "# My rules\n\nAlways write tests.\n"
	if err := os.WriteFile(filepath.Join(memDir, windsurfRulesFile), []byte(userRules), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := (windsurf{}).Install(); err != nil {
		t.Fatalf("Install() error = %v", err)
	}

	got, err := os.ReadFile(filepath.Join(memDir, windsurfRulesFile))
	if err != nil {
		t.Fatal(err)
	}
	body := string(got)
	if !strings.Contains(body, "Always write tests.") {
		t.Errorf("user rules were lost; body=%q", body)
	}
	if !strings.Contains(body, sectionEndMarker) {
		t.Errorf("DeployHQ section not appended; body=%q", body)
	}
}

func TestWindsurf_Install_ReplacesExistingSection(t *testing.T) {
	home := t.TempDir()
	withHomeDir(t, home)
	memDir := filepath.Join(home, ".codeium", "windsurf", "memories")
	if err := os.MkdirAll(memDir, 0o755); err != nil {
		t.Fatal(err)
	}
	// Pre-existing rules with a stale DeployHQ section sandwiched between
	// user rules. Both halves must survive; the section in the middle gets
	// rewritten.
	pre := "# Top rule\n\n<!-- BEGIN deployhq-skill v0 -->\nold stale content here\n<!-- END deployhq-skill -->\n\n# Bottom rule\n"
	if err := os.WriteFile(filepath.Join(memDir, windsurfRulesFile), []byte(pre), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := (windsurf{}).Install(); err != nil {
		t.Fatalf("Install() error = %v", err)
	}

	got, err := os.ReadFile(filepath.Join(memDir, windsurfRulesFile))
	if err != nil {
		t.Fatal(err)
	}
	body := string(got)
	for _, must := range []string{"# Top rule", "# Bottom rule", sectionBeginPrefix + skills.Version + sectionBeginSuffix} {
		if !strings.Contains(body, must) {
			t.Errorf("missing %q in merged body:\n%s", must, body)
		}
	}
	if strings.Contains(body, "old stale content here") {
		t.Errorf("stale section content survived:\n%s", body)
	}
	if strings.Contains(body, "v0 -->") {
		t.Errorf("stale BEGIN marker survived:\n%s", body)
	}
}

func TestWindsurf_Detect_InstalledAndOutdated(t *testing.T) {
	home := t.TempDir()
	withHomeDir(t, home)
	if err := os.MkdirAll(filepath.Join(home, ".codeium", "windsurf"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := (windsurf{}).Install(); err != nil {
		t.Fatal(err)
	}
	if got := (windsurf{}).Detect(); got != StatusInstalled {
		t.Fatalf("after install Detect() = %v, want StatusInstalled", got)
	}

	// Replace BEGIN marker with an older version.
	rulesPath := filepath.Join(home, ".codeium", "windsurf", "memories", windsurfRulesFile)
	body, err := os.ReadFile(rulesPath)
	if err != nil {
		t.Fatal(err)
	}
	stale := strings.Replace(string(body), sectionBeginPrefix+skills.Version, sectionBeginPrefix+"0", 1)
	if err := os.WriteFile(rulesPath, []byte(stale), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := (windsurf{}).Detect(); got != StatusOutdated {
		t.Fatalf("stale version Detect() = %v, want StatusOutdated", got)
	}
}

func TestWindsurf_Install_Idempotent(t *testing.T) {
	home := t.TempDir()
	withHomeDir(t, home)
	if err := os.MkdirAll(filepath.Join(home, ".codeium", "windsurf"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := (windsurf{}).Install(); err != nil {
		t.Fatal(err)
	}
	rulesPath := filepath.Join(home, ".codeium", "windsurf", "memories", windsurfRulesFile)
	first, err := os.ReadFile(rulesPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := (windsurf{}).Install(); err != nil {
		t.Fatalf("second Install() = %v", err)
	}
	second, err := os.ReadFile(rulesPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(first) != string(second) {
		t.Errorf("re-install produced a different file\n--- first ---\n%s\n--- second ---\n%s", first, second)
	}
	if got := (windsurf{}).Detect(); got != StatusInstalled {
		t.Fatalf("after re-install Detect() = %v, want StatusInstalled", got)
	}
}

func TestRegistry_ContainsWindsurf(t *testing.T) {
	if Find("windsurf") == nil {
		t.Fatal("windsurf target not registered")
	}
}
