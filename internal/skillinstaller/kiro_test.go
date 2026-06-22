package skillinstaller

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/deployhq/deployhq-cli/skills"
)

func TestKiro_Detect_NotInRepo(t *testing.T) {
	withCwd(t, t.TempDir())
	if got := (kiro{}).Detect(); got != StatusNotInstalled {
		t.Fatalf("Detect() outside repo = %v, want StatusNotInstalled", got)
	}
}

func TestKiro_Detect_InRepoNoSkill(t *testing.T) {
	withCwd(t, fakeRepo(t))
	if got := (kiro{}).Detect(); got != StatusAvailable {
		t.Fatalf("Detect() = %v, want StatusAvailable", got)
	}
}

func TestKiro_Install_OutsideRepo_Errors(t *testing.T) {
	withCwd(t, t.TempDir())
	if _, err := (kiro{}).Install(); err == nil {
		t.Fatal("expected error outside repo")
	}
}

func TestKiro_Install_WritesSkillFile(t *testing.T) {
	dir := fakeRepo(t)
	withCwd(t, dir)

	got, err := (kiro{}).Install()
	if err != nil {
		t.Fatalf("Install() = %v", err)
	}
	want := filepath.Join(dir, kiroSteeringDir, kiroSkillFile)
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
		t.Error("SKILL.md body missing")
	}
}

func TestKiro_Install_CoexistsWithOtherSteeringFiles(t *testing.T) {
	dir := fakeRepo(t)
	withCwd(t, dir)
	// User already has steering files; ours must land alongside without
	// disturbing theirs.
	steering := filepath.Join(dir, kiroSteeringDir)
	if err := os.MkdirAll(steering, 0o755); err != nil {
		t.Fatal(err)
	}
	other := filepath.Join(steering, "team-rules.md")
	if err := os.WriteFile(other, []byte("# Team rules\nbe nice\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := (kiro{}).Install(); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(other)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "# Team rules\nbe nice\n" {
		t.Errorf("user steering file was disturbed: %q", got)
	}
}

func TestKiro_Detect_InstalledThenOutdated(t *testing.T) {
	dir := fakeRepo(t)
	withCwd(t, dir)
	if _, err := (kiro{}).Install(); err != nil {
		t.Fatal(err)
	}
	if got := (kiro{}).Detect(); got != StatusInstalled {
		t.Fatalf("post-install Detect() = %v, want StatusInstalled", got)
	}

	path := filepath.Join(dir, kiroSteeringDir, kiroSkillFile)
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	stale := strings.Replace(string(body), ownedFileVersionPrefix+skills.Version, ownedFileVersionPrefix+"0", 1)
	if err := os.WriteFile(path, []byte(stale), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := (kiro{}).Detect(); got != StatusOutdated {
		t.Fatalf("Detect() stale marker = %v, want StatusOutdated", got)
	}
}

func TestKiro_Install_Idempotent(t *testing.T) {
	dir := fakeRepo(t)
	withCwd(t, dir)

	if _, err := (kiro{}).Install(); err != nil {
		t.Fatal(err)
	}
	first, err := os.ReadFile(filepath.Join(dir, kiroSteeringDir, kiroSkillFile))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := (kiro{}).Install(); err != nil {
		t.Fatalf("second Install() = %v", err)
	}
	second, err := os.ReadFile(filepath.Join(dir, kiroSteeringDir, kiroSkillFile))
	if err != nil {
		t.Fatal(err)
	}
	if string(first) != string(second) {
		t.Errorf("idempotence broken\n--- first ---\n%s\n--- second ---\n%s", first, second)
	}
}

func TestKiro_Scope_IsProject(t *testing.T) {
	if got := (kiro{}).Scope(); got != ScopeProject {
		t.Errorf("Scope() = %v, want ScopeProject", got)
	}
}

func TestRegistry_ContainsKiro(t *testing.T) {
	if Find("kiro") == nil {
		t.Fatal("kiro target not registered")
	}
}
