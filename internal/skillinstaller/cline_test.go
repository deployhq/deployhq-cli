package skillinstaller

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/deployhq/deployhq-cli/skills"
)

func TestCline_Detect_NotInRepo(t *testing.T) {
	withCwd(t, t.TempDir())
	if got := (cline{}).Detect(); got != StatusNotInstalled {
		t.Fatalf("Detect() outside repo = %v, want StatusNotInstalled", got)
	}
}

func TestCline_Detect_InRepoNoRules(t *testing.T) {
	withCwd(t, fakeRepo(t))
	if got := (cline{}).Detect(); got != StatusAvailable {
		t.Fatalf("Detect() = %v, want StatusAvailable", got)
	}
}

func TestCline_Detect_LegacyFileShape(t *testing.T) {
	dir := fakeRepo(t)
	withCwd(t, dir)
	// .clinerules exists as a single file (legacy shape).
	if err := os.WriteFile(filepath.Join(dir, clineRulesDir), []byte("user rule\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Detect still reports Available so it shows up in `dhq skills list`;
	// Install will refuse with an actionable message.
	if got := (cline{}).Detect(); got != StatusAvailable {
		t.Fatalf("Detect() with legacy file = %v, want StatusAvailable", got)
	}
}

func TestCline_Install_OutsideRepo_Errors(t *testing.T) {
	withCwd(t, t.TempDir())
	if _, err := (cline{}).Install(); err == nil {
		t.Fatal("expected error outside repo")
	}
}

func TestCline_Install_LegacyFileShape_Errors(t *testing.T) {
	dir := fakeRepo(t)
	withCwd(t, dir)
	if err := os.WriteFile(filepath.Join(dir, clineRulesDir), []byte("legacy\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := (cline{}).Install()
	if err == nil {
		t.Fatal("expected error when .clinerules is a file")
	}
	if !strings.Contains(err.Error(), "legacy Cline single-rule shape") {
		t.Errorf("expected actionable migration hint, got: %v", err)
	}
}

func TestCline_Install_WritesSkillFile(t *testing.T) {
	dir := fakeRepo(t)
	withCwd(t, dir)

	got, err := (cline{}).Install()
	if err != nil {
		t.Fatalf("Install() = %v", err)
	}
	want := filepath.Join(dir, clineRulesDir, clineSkillFile)
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
}

func TestCline_Detect_InstalledThenOutdated(t *testing.T) {
	dir := fakeRepo(t)
	withCwd(t, dir)
	if _, err := (cline{}).Install(); err != nil {
		t.Fatal(err)
	}
	if got := (cline{}).Detect(); got != StatusInstalled {
		t.Fatalf("post-install Detect() = %v, want StatusInstalled", got)
	}

	path := filepath.Join(dir, clineRulesDir, clineSkillFile)
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	stale := strings.Replace(string(body), ownedFileVersionPrefix+skills.Version, ownedFileVersionPrefix+"0", 1)
	if err := os.WriteFile(path, []byte(stale), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := (cline{}).Detect(); got != StatusOutdated {
		t.Fatalf("Detect() stale marker = %v, want StatusOutdated", got)
	}
}

func TestCline_Install_Idempotent(t *testing.T) {
	dir := fakeRepo(t)
	withCwd(t, dir)

	if _, err := (cline{}).Install(); err != nil {
		t.Fatal(err)
	}
	first, err := os.ReadFile(filepath.Join(dir, clineRulesDir, clineSkillFile))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := (cline{}).Install(); err != nil {
		t.Fatalf("second Install() = %v", err)
	}
	second, err := os.ReadFile(filepath.Join(dir, clineRulesDir, clineSkillFile))
	if err != nil {
		t.Fatal(err)
	}
	if string(first) != string(second) {
		t.Errorf("idempotence broken\n--- first ---\n%s\n--- second ---\n%s", first, second)
	}
}

func TestCline_Scope_IsProject(t *testing.T) {
	if got := (cline{}).Scope(); got != ScopeProject {
		t.Errorf("Scope() = %v, want ScopeProject", got)
	}
}

func TestRegistry_ContainsCline(t *testing.T) {
	if Find("cline") == nil {
		t.Fatal("cline target not registered")
	}
}
