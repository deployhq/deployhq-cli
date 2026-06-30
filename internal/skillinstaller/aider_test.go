package skillinstaller

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/deployhq/deployhq-cli/skills"
)

// withAiderPresent overrides the binary-on-PATH check for the duration of
// the test. We avoid depending on whether the dev machine actually has
// aider installed.
func withAiderPresent(t *testing.T, present bool) {
	t.Helper()
	orig := findAider
	findAider = func() bool { return present }
	t.Cleanup(func() { findAider = orig })
}

func TestAider_Detect_NoBinary(t *testing.T) {
	withHomeDir(t, t.TempDir())
	withAiderPresent(t, false)
	if got := (aider{}).Detect(); got != StatusNotInstalled {
		t.Fatalf("Detect() without aider on PATH = %v, want StatusNotInstalled", got)
	}
}

func TestAider_Detect_BinaryPresentNoSkill(t *testing.T) {
	withHomeDir(t, t.TempDir())
	withAiderPresent(t, true)
	if got := (aider{}).Detect(); got != StatusAvailable {
		t.Fatalf("Detect() = %v, want StatusAvailable", got)
	}
}

func TestAider_Install_WritesVersionedFile(t *testing.T) {
	home := t.TempDir()
	withHomeDir(t, home)
	withAiderPresent(t, true)

	got, err := (aider{}).Install()
	if err != nil {
		t.Fatalf("Install() = %v", err)
	}
	want := filepath.Join(home, aiderSkillDir, aiderSkillFile)
	if got != want {
		t.Errorf("Install() path = %q, want %q", got, want)
	}

	body, err := os.ReadFile(want)
	if err != nil {
		t.Fatal(err)
	}
	s := string(body)
	if !strings.HasPrefix(s, ownedFileVersionPrefix+skills.Version+ownedFileVersionSuffix+"\n") {
		t.Errorf("file must start with version marker; got:\n%.120q", s)
	}
	// SKILL.md body should be present after the marker.
	if !strings.Contains(s, "DeployHQ CLI — Agent Skill Guide") {
		t.Error("SKILL.md body missing from output")
	}
	// References should be inlined.
	if !strings.Contains(s, "## reference: references/") {
		t.Error("references not concatenated into output")
	}
}

func TestAider_Detect_InstalledThenOutdated(t *testing.T) {
	home := t.TempDir()
	withHomeDir(t, home)
	withAiderPresent(t, true)

	if _, err := (aider{}).Install(); err != nil {
		t.Fatal(err)
	}
	if got := (aider{}).Detect(); got != StatusInstalled {
		t.Fatalf("post-install Detect() = %v, want StatusInstalled", got)
	}

	// Tweak the marker to simulate an older version.
	path := filepath.Join(home, aiderSkillDir, aiderSkillFile)
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	stale := strings.Replace(string(body), ownedFileVersionPrefix+skills.Version, ownedFileVersionPrefix+"0", 1)
	if err := os.WriteFile(path, []byte(stale), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := (aider{}).Detect(); got != StatusOutdated {
		t.Fatalf("Detect() stale marker = %v, want StatusOutdated", got)
	}
}

func TestAider_Detect_FileWithoutMarkerIsAvailable(t *testing.T) {
	home := t.TempDir()
	withHomeDir(t, home)
	withAiderPresent(t, true)

	dir := filepath.Join(home, aiderSkillDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, aiderSkillFile), []byte("user wrote this\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// We can't assume an unmarked file is ours — Detect() returns Available
	// so the user can confirm and we'll overwrite with a versioned install.
	if got := (aider{}).Detect(); got != StatusAvailable {
		t.Fatalf("unmarked file Detect() = %v, want StatusAvailable", got)
	}
}

func TestAider_Install_Idempotent(t *testing.T) {
	home := t.TempDir()
	withHomeDir(t, home)
	withAiderPresent(t, true)

	if _, err := (aider{}).Install(); err != nil {
		t.Fatal(err)
	}
	first, err := os.ReadFile(filepath.Join(home, aiderSkillDir, aiderSkillFile))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := (aider{}).Install(); err != nil {
		t.Fatalf("second Install() = %v", err)
	}
	second, err := os.ReadFile(filepath.Join(home, aiderSkillDir, aiderSkillFile))
	if err != nil {
		t.Fatal(err)
	}
	if string(first) != string(second) {
		t.Errorf("idempotence broken\n--- first ---\n%s\n--- second ---\n%s", first, second)
	}
}

func TestAider_PostInstallNote_MentionsExactPath(t *testing.T) {
	home := t.TempDir()
	withHomeDir(t, home)

	note := (aider{}).PostInstallNote()
	if note == "" {
		t.Fatal("PostInstallNote() returned empty string")
	}
	// Compare against the quoted forms the note actually emits. On Linux
	// the raw path would also be a substring (the quoting is a near no-op
	// without metacharacters), but on Windows the path has backslashes that
	// get escaped — so we'd never find the raw path. Always compare against
	// the quoted forms to stay portable. The YAML and shell snippets use
	// different quoting, so check both.
	skillPath := filepath.Join(home, aiderSkillDir, aiderSkillFile)
	wantYAML := quotePathForYAML(skillPath)
	if !strings.Contains(note, wantYAML) {
		t.Errorf("note doesn't mention YAML-quoted path %s in %s", wantYAML, note)
	}
	wantShell := quotePathForShell(skillPath)
	if !strings.Contains(note, wantShell) {
		t.Errorf("note doesn't mention shell-quoted path %s in %s", wantShell, note)
	}
	if !strings.Contains(note, ".aider.conf.yml") {
		t.Errorf("note should point at .aider.conf.yml: %s", note)
	}
}

func TestAider_ImplementsNoter(t *testing.T) {
	var _ Noter = (*aider)(nil)
}

func TestAider_Scope_IsUser(t *testing.T) {
	if got := (aider{}).Scope(); got != ScopeUser {
		t.Errorf("Scope() = %v, want ScopeUser", got)
	}
}

func TestRegistry_ContainsAider(t *testing.T) {
	if Find("aider") == nil {
		t.Fatal("aider target not registered")
	}
}

func TestParseOwnedFileVersion(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"empty", "", ""},
		{"no marker", "# user content\n", ""},
		{"v1 at start", "<!-- deployhq-skill v1 -->\nbody\n", "1"},
		// Owned-file contract: marker must be the first thing in the
		// file. Anything above it means the file isn't ours and we
		// should rewrite, not trust it.
		{"marker not on first line is rejected", "x\n<!-- deployhq-skill v42 -->\nbody\n", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := parseOwnedFileVersion(tc.in); got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}
