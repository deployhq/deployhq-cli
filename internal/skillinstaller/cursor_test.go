package skillinstaller

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/deployhq/deployhq-cli/skills"
)

func TestCursor_Detect_NoConfigDir(t *testing.T) {
	withHomeDir(t, t.TempDir())
	if got := (cursor{}).Detect(); got != StatusNotInstalled {
		t.Fatalf("Detect() = %v, want StatusNotInstalled", got)
	}
}

func TestCursor_Detect_AgentInstalledSkillMissing(t *testing.T) {
	home := t.TempDir()
	withHomeDir(t, home)
	if err := os.MkdirAll(filepath.Join(home, ".cursor"), 0o755); err != nil {
		t.Fatal(err)
	}
	if got := (cursor{}).Detect(); got != StatusAvailable {
		t.Fatalf("Detect() = %v, want StatusAvailable", got)
	}
}

func TestCursor_Install_WritesMDC(t *testing.T) {
	home := t.TempDir()
	withHomeDir(t, home)
	if err := os.MkdirAll(filepath.Join(home, ".cursor"), 0o755); err != nil {
		t.Fatal(err)
	}

	path, err := (cursor{}).Install()
	if err != nil {
		t.Fatalf("Install() error = %v", err)
	}

	wantPath := filepath.Join(home, ".cursor", "rules", "deployhq.mdc")
	if path != wantPath {
		t.Errorf("Install() path = %q, want %q", path, wantPath)
	}

	content, err := os.ReadFile(wantPath)
	if err != nil {
		t.Fatalf("read mdc: %v", err)
	}
	body := string(content)

	if !strings.HasPrefix(body, "---\n") {
		t.Error("mdc missing leading frontmatter delimiter")
	}
	if !strings.Contains(body, "alwaysApply: false") {
		t.Error("mdc missing alwaysApply: false")
	}
	// The description from SKILL.md ("Deploy code…") should make it into
	// the Cursor frontmatter, collapsed to one line and YAML-quoted.
	if !strings.Contains(body, `description: "Deploy code`) {
		t.Errorf("mdc missing quoted description extracted from SKILL.md\n--- body ---\n%s", body[:min(400, len(body))])
	}
	// SKILL.md body content should appear (post-frontmatter heading).
	if !strings.Contains(body, "DeployHQ CLI — Agent Skill Guide") {
		t.Error("mdc missing SKILL.md body content")
	}
	// References must be inlined under a discoverable header.
	if !strings.Contains(body, "## reference: references/") {
		t.Error("mdc missing reference sections")
	}

	v, err := os.ReadFile(filepath.Join(home, ".cursor", "rules", versionMarker))
	if err != nil {
		t.Fatalf("read version marker: %v", err)
	}
	if strings.TrimSpace(string(v)) != skills.Version {
		t.Errorf("version marker = %q, want %q", string(v), skills.Version)
	}
}

func TestCursor_Detect_InstalledThenOutdated(t *testing.T) {
	home := t.TempDir()
	withHomeDir(t, home)
	if err := os.MkdirAll(filepath.Join(home, ".cursor"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := (cursor{}).Install(); err != nil {
		t.Fatal(err)
	}
	if got := (cursor{}).Detect(); got != StatusInstalled {
		t.Fatalf("after install Detect() = %v, want StatusInstalled", got)
	}
	// Force-stale the version marker.
	if err := os.WriteFile(
		filepath.Join(home, ".cursor", "rules", versionMarker),
		[]byte("0\n"), 0o644,
	); err != nil {
		t.Fatal(err)
	}
	if got := (cursor{}).Detect(); got != StatusOutdated {
		t.Fatalf("with stale marker Detect() = %v, want StatusOutdated", got)
	}
}

func TestCursor_Install_Idempotent(t *testing.T) {
	home := t.TempDir()
	withHomeDir(t, home)
	if err := os.MkdirAll(filepath.Join(home, ".cursor"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := (cursor{}).Install(); err != nil {
		t.Fatal(err)
	}
	if _, err := (cursor{}).Install(); err != nil {
		t.Fatalf("second Install() = %v", err)
	}
	if got := (cursor{}).Detect(); got != StatusInstalled {
		t.Fatalf("after re-install Detect() = %v, want StatusInstalled", got)
	}
}

func TestExtractDescription(t *testing.T) {
	cases := []struct {
		name string
		fm   string
		want string
	}{
		{
			name: "inline",
			fm:   "description: hello world\nother: x",
			want: "hello world",
		},
		{
			name: "literal block",
			fm:   "name: foo\ndescription: |\n  Line one of the description.\n  Line two of the description.\nlicense: MIT",
			want: "Line one of the description. Line two of the description.",
		},
		{
			name: "missing",
			fm:   "name: foo\nlicense: MIT",
			want: "",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := extractDescription(tc.fm); got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestRegistry_ContainsCursor(t *testing.T) {
	if Find("cursor") == nil {
		t.Fatal("cursor target not registered")
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
