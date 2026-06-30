package skillinstaller

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/deployhq/deployhq-cli/skills"
)

func TestContinue_Detect_NoConfigDir(t *testing.T) {
	withHomeDir(t, t.TempDir())
	if got := (continueDev{}).Detect(); got != StatusNotInstalled {
		t.Fatalf("Detect() = %v, want StatusNotInstalled", got)
	}
}

func TestContinue_Detect_InstalledNoSkillFile(t *testing.T) {
	home := t.TempDir()
	withHomeDir(t, home)
	if err := os.MkdirAll(filepath.Join(home, continueConfigDir), 0o755); err != nil {
		t.Fatal(err)
	}
	if got := (continueDev{}).Detect(); got != StatusAvailable {
		t.Fatalf("Detect() = %v, want StatusAvailable", got)
	}
}

func TestContinue_Install_WritesVersionedFile(t *testing.T) {
	home := t.TempDir()
	withHomeDir(t, home)
	if err := os.MkdirAll(filepath.Join(home, continueConfigDir), 0o755); err != nil {
		t.Fatal(err)
	}

	got, err := (continueDev{}).Install()
	if err != nil {
		t.Fatalf("Install() = %v", err)
	}
	want := filepath.Join(home, continueConfigDir, continueRulesDir, continueSkillFile)
	if got != want {
		t.Errorf("Install() path = %q, want %q", got, want)
	}

	body, err := os.ReadFile(want)
	if err != nil {
		t.Fatal(err)
	}
	s := string(body)
	if !strings.HasPrefix(s, ownedFileVersionPrefix+skills.Version+ownedFileVersionSuffix+"\n") {
		t.Errorf("file must start with version marker; got: %.120q", s)
	}
	if !strings.Contains(s, "DeployHQ CLI — Agent Skill Guide") {
		t.Error("SKILL.md body missing from output")
	}
	if !strings.Contains(s, "## reference: references/") {
		t.Error("references not concatenated into output")
	}
}

func TestContinue_Detect_InstalledThenOutdated(t *testing.T) {
	home := t.TempDir()
	withHomeDir(t, home)
	if err := os.MkdirAll(filepath.Join(home, continueConfigDir), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := (continueDev{}).Install(); err != nil {
		t.Fatal(err)
	}
	if got := (continueDev{}).Detect(); got != StatusInstalled {
		t.Fatalf("post-install Detect() = %v, want StatusInstalled", got)
	}

	path := filepath.Join(home, continueConfigDir, continueRulesDir, continueSkillFile)
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	stale := strings.Replace(string(body), ownedFileVersionPrefix+skills.Version, ownedFileVersionPrefix+"0", 1)
	if err := os.WriteFile(path, []byte(stale), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := (continueDev{}).Detect(); got != StatusOutdated {
		t.Fatalf("Detect() stale marker = %v, want StatusOutdated", got)
	}
}

func TestContinue_Install_Idempotent(t *testing.T) {
	home := t.TempDir()
	withHomeDir(t, home)
	if err := os.MkdirAll(filepath.Join(home, continueConfigDir), 0o755); err != nil {
		t.Fatal(err)
	}

	if _, err := (continueDev{}).Install(); err != nil {
		t.Fatal(err)
	}
	first, err := os.ReadFile(filepath.Join(home, continueConfigDir, continueRulesDir, continueSkillFile))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := (continueDev{}).Install(); err != nil {
		t.Fatalf("second Install() = %v", err)
	}
	second, err := os.ReadFile(filepath.Join(home, continueConfigDir, continueRulesDir, continueSkillFile))
	if err != nil {
		t.Fatal(err)
	}
	if string(first) != string(second) {
		t.Errorf("idempotence broken\n--- first ---\n%s\n--- second ---\n%s", first, second)
	}
}

func TestContinue_Scope_IsUser(t *testing.T) {
	if got := (continueDev{}).Scope(); got != ScopeUser {
		t.Errorf("Scope() = %v, want ScopeUser", got)
	}
}

func TestRegistry_ContainsContinue(t *testing.T) {
	if Find("continue") == nil {
		t.Fatal("continue target not registered")
	}
}
