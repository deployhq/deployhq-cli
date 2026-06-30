package skillinstaller

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/deployhq/deployhq-cli/skills"
)

func TestCodex_Detect_NoConfigDir(t *testing.T) {
	withHomeDir(t, t.TempDir())
	if got := (codex{}).Detect(); got != StatusNotInstalled {
		t.Fatalf("Detect() = %v, want StatusNotInstalled", got)
	}
}

func TestCodex_Detect_InstalledNoAgentsFile(t *testing.T) {
	home := t.TempDir()
	withHomeDir(t, home)
	if err := os.MkdirAll(filepath.Join(home, ".codex"), 0o755); err != nil {
		t.Fatal(err)
	}
	if got := (codex{}).Detect(); got != StatusAvailable {
		t.Fatalf("Detect() = %v, want StatusAvailable", got)
	}
}

func TestCodex_Detect_AgentsFileWithoutSection(t *testing.T) {
	home := t.TempDir()
	withHomeDir(t, home)
	cfg := filepath.Join(home, ".codex")
	if err := os.MkdirAll(cfg, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cfg, codexAgentsFile), []byte("# User instructions\nbe concise\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := (codex{}).Detect(); got != StatusAvailable {
		t.Fatalf("Detect() with unrelated content = %v, want StatusAvailable", got)
	}
}

func TestCodex_Install_WritesSectionAndRefs(t *testing.T) {
	home := t.TempDir()
	withHomeDir(t, home)
	if err := os.MkdirAll(filepath.Join(home, ".codex"), 0o755); err != nil {
		t.Fatal(err)
	}

	got, err := (codex{}).Install()
	if err != nil {
		t.Fatalf("Install() = %v", err)
	}
	want := filepath.Join(home, ".codex", codexAgentsFile)
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
		filepath.Join(home, ".codex", codexRefsDir, "SKILL.md"),
	} {
		if !strings.Contains(string(body), must) {
			t.Errorf("AGENTS.md missing %q\n%s", must, body)
		}
	}

	if _, err := os.Stat(filepath.Join(home, ".codex", codexRefsDir, "SKILL.md")); err != nil {
		t.Errorf("expected SKILL.md under refs root: %v", err)
	}
	refs, err := os.ReadDir(filepath.Join(home, ".codex", codexRefsDir, "references"))
	if err != nil || len(refs) == 0 {
		t.Errorf("expected references/*.md: err=%v entries=%d", err, len(refs))
	}
}

func TestCodex_Install_PreservesUserContent(t *testing.T) {
	home := t.TempDir()
	withHomeDir(t, home)
	cfg := filepath.Join(home, ".codex")
	if err := os.MkdirAll(cfg, 0o755); err != nil {
		t.Fatal(err)
	}
	userContent := "# My agent rules\n\nAlways explain changes.\n"
	if err := os.WriteFile(filepath.Join(cfg, codexAgentsFile), []byte(userContent), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := (codex{}).Install(); err != nil {
		t.Fatalf("Install() = %v", err)
	}
	got, err := os.ReadFile(filepath.Join(cfg, codexAgentsFile))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(got), "Always explain changes.") {
		t.Errorf("user content lost:\n%s", got)
	}
	if !strings.Contains(string(got), sectionEndMarker) {
		t.Errorf("DeployHQ section not appended:\n%s", got)
	}
}

func TestCodex_Install_Idempotent(t *testing.T) {
	home := t.TempDir()
	withHomeDir(t, home)
	if err := os.MkdirAll(filepath.Join(home, ".codex"), 0o755); err != nil {
		t.Fatal(err)
	}

	if _, err := (codex{}).Install(); err != nil {
		t.Fatal(err)
	}
	first, err := os.ReadFile(filepath.Join(home, ".codex", codexAgentsFile))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := (codex{}).Install(); err != nil {
		t.Fatalf("second Install() = %v", err)
	}
	second, err := os.ReadFile(filepath.Join(home, ".codex", codexAgentsFile))
	if err != nil {
		t.Fatal(err)
	}
	if string(first) != string(second) {
		t.Errorf("idempotence broken\n--- first ---\n%s\n--- second ---\n%s", first, second)
	}
}

func TestCodex_Detect_Outdated(t *testing.T) {
	home := t.TempDir()
	withHomeDir(t, home)
	if err := os.MkdirAll(filepath.Join(home, ".codex"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := (codex{}).Install(); err != nil {
		t.Fatal(err)
	}
	if got := (codex{}).Detect(); got != StatusInstalled {
		t.Fatalf("post-install Detect() = %v, want StatusInstalled", got)
	}

	path := filepath.Join(home, ".codex", codexAgentsFile)
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	stale := strings.Replace(string(body), sectionBeginPrefix+skills.Version, sectionBeginPrefix+"0", 1)
	if err := os.WriteFile(path, []byte(stale), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := (codex{}).Detect(); got != StatusOutdated {
		t.Fatalf("Detect() stale marker = %v, want StatusOutdated", got)
	}
}

func TestCodex_Scope_IsUser(t *testing.T) {
	if got := (codex{}).Scope(); got != ScopeUser {
		t.Errorf("Scope() = %v, want ScopeUser", got)
	}
}

func TestRegistry_ContainsCodex(t *testing.T) {
	if Find("codex") == nil {
		t.Fatal("codex target not registered")
	}
}
