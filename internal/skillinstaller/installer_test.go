package skillinstaller

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/deployhq/deployhq-cli/skills"
)

// withHomeDir swaps homeDir for the duration of the test. The Claude target
// reads ~/.claude — we point it at a t.TempDir so tests don't touch the real
// user home.
func withHomeDir(t *testing.T, dir string) {
	t.Helper()
	orig := homeDir
	homeDir = func() (string, error) { return dir, nil }
	t.Cleanup(func() { homeDir = orig })
}

func TestClaudeCode_Detect_NoConfigDir(t *testing.T) {
	withHomeDir(t, t.TempDir())
	if got := (claudeCode{}).Detect(); got != StatusNotInstalled {
		t.Fatalf("Detect() = %v, want StatusNotInstalled", got)
	}
}

func TestClaudeCode_Detect_AgentInstalledSkillMissing(t *testing.T) {
	home := t.TempDir()
	withHomeDir(t, home)
	// Simulate Claude Code installed (just the config dir exists).
	if err := os.MkdirAll(filepath.Join(home, ".claude"), 0o755); err != nil {
		t.Fatal(err)
	}
	if got := (claudeCode{}).Detect(); got != StatusAvailable {
		t.Fatalf("Detect() = %v, want StatusAvailable", got)
	}
}

func TestClaudeCode_Install_WritesTreeAndVersion(t *testing.T) {
	home := t.TempDir()
	withHomeDir(t, home)
	if err := os.MkdirAll(filepath.Join(home, ".claude"), 0o755); err != nil {
		t.Fatal(err)
	}

	path, err := (claudeCode{}).Install()
	if err != nil {
		t.Fatalf("Install() error = %v", err)
	}

	wantDir := filepath.Join(home, ".claude", "skills", "deployhq")
	if path != wantDir {
		t.Errorf("Install() path = %q, want %q", path, wantDir)
	}

	// SKILL.md must exist and be non-empty.
	skill, err := os.ReadFile(filepath.Join(wantDir, "SKILL.md"))
	if err != nil {
		t.Fatalf("read SKILL.md: %v", err)
	}
	if !strings.Contains(string(skill), "name: deployhq") {
		t.Errorf("SKILL.md content unexpected: %.80q", string(skill))
	}

	// At least one reference file must have been written.
	refs, err := os.ReadDir(filepath.Join(wantDir, "references"))
	if err != nil {
		t.Fatalf("read references/: %v", err)
	}
	if len(refs) == 0 {
		t.Error("references/ is empty after install")
	}

	// Version marker must match skills.Version.
	v, err := os.ReadFile(filepath.Join(wantDir, versionMarker))
	if err != nil {
		t.Fatalf("read version marker: %v", err)
	}
	if strings.TrimSpace(string(v)) != skills.Version {
		t.Errorf("version marker = %q, want %q", string(v), skills.Version)
	}
}

func TestClaudeCode_Detect_InstalledMatchesVersion(t *testing.T) {
	home := t.TempDir()
	withHomeDir(t, home)
	if err := os.MkdirAll(filepath.Join(home, ".claude"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := (claudeCode{}).Install(); err != nil {
		t.Fatal(err)
	}
	if got := (claudeCode{}).Detect(); got != StatusInstalled {
		t.Fatalf("Detect() after install = %v, want StatusInstalled", got)
	}
}

func TestClaudeCode_Detect_OutdatedVersionMarker(t *testing.T) {
	home := t.TempDir()
	withHomeDir(t, home)
	if err := os.MkdirAll(filepath.Join(home, ".claude"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := (claudeCode{}).Install(); err != nil {
		t.Fatal(err)
	}
	// Overwrite version marker with an older version.
	if err := os.WriteFile(
		filepath.Join(home, ".claude", "skills", "deployhq", versionMarker),
		[]byte("0\n"), 0o644,
	); err != nil {
		t.Fatal(err)
	}
	if got := (claudeCode{}).Detect(); got != StatusOutdated {
		t.Fatalf("Detect() with stale marker = %v, want StatusOutdated", got)
	}
}

func TestClaudeCode_Install_Idempotent(t *testing.T) {
	home := t.TempDir()
	withHomeDir(t, home)
	if err := os.MkdirAll(filepath.Join(home, ".claude"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := (claudeCode{}).Install(); err != nil {
		t.Fatal(err)
	}
	// Second install must not error and must leave the same files in place.
	if _, err := (claudeCode{}).Install(); err != nil {
		t.Fatalf("second Install() = %v", err)
	}
	if got := (claudeCode{}).Detect(); got != StatusInstalled {
		t.Fatalf("after re-install Detect() = %v, want StatusInstalled", got)
	}
}

func TestRegistry_ContainsClaudeCode(t *testing.T) {
	if Find("claude-code") == nil {
		t.Fatal("claude-code target not registered")
	}
}

func TestDetectInstalled_FiltersNotInstalled(t *testing.T) {
	withHomeDir(t, t.TempDir())
	// With an empty home, claude target should not appear in DetectInstalled.
	got := DetectInstalled()
	for _, r := range got {
		if r.Target.Name() == "claude-code" {
			t.Errorf("claude-code returned by DetectInstalled() with empty home: status=%v", r.Status)
		}
	}
}

func TestNeeded(t *testing.T) {
	cases := []struct {
		s    Status
		want bool
	}{
		{StatusNotInstalled, false},
		{StatusAvailable, true},
		{StatusOutdated, true},
		{StatusInstalled, false},
	}
	for _, tc := range cases {
		if got := Needed(tc.s); got != tc.want {
			t.Errorf("Needed(%v) = %v, want %v", tc.s, got, tc.want)
		}
	}
}
