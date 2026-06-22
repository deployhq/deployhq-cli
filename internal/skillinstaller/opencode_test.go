package skillinstaller

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/deployhq/deployhq-cli/skills"
)

func TestOpenCode_Detect_NoConfigDir(t *testing.T) {
	withHomeDir(t, t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", "")
	if got := (opencode{}).Detect(); got != StatusNotInstalled {
		t.Fatalf("Detect() = %v, want StatusNotInstalled", got)
	}
}

func TestOpenCode_Detect_RespectsXDG(t *testing.T) {
	home := t.TempDir()
	xdg := t.TempDir()
	withHomeDir(t, home)
	t.Setenv("XDG_CONFIG_HOME", xdg)

	// Without the XDG opencode/ dir, detect is NotInstalled even though
	// ~/.config/opencode might exist somewhere — XDG_CONFIG_HOME wins.
	if got := (opencode{}).Detect(); got != StatusNotInstalled {
		t.Fatalf("Detect() with empty XDG dir = %v, want StatusNotInstalled", got)
	}

	// Create the OpenCode config dir under XDG_CONFIG_HOME → Available.
	if err := os.MkdirAll(filepath.Join(xdg, "opencode"), 0o755); err != nil {
		t.Fatal(err)
	}
	if got := (opencode{}).Detect(); got != StatusAvailable {
		t.Fatalf("Detect() with XDG opencode dir = %v, want StatusAvailable", got)
	}
}

func TestOpenCode_Detect_FallsBackTo_DotConfig(t *testing.T) {
	home := t.TempDir()
	withHomeDir(t, home)
	t.Setenv("XDG_CONFIG_HOME", "")

	if err := os.MkdirAll(filepath.Join(home, ".config", "opencode"), 0o755); err != nil {
		t.Fatal(err)
	}
	if got := (opencode{}).Detect(); got != StatusAvailable {
		t.Fatalf("Detect() in ~/.config/opencode = %v, want StatusAvailable", got)
	}
}

func TestOpenCode_Install_WritesSectionAndRefs(t *testing.T) {
	home := t.TempDir()
	withHomeDir(t, home)
	t.Setenv("XDG_CONFIG_HOME", "")
	cfg := filepath.Join(home, ".config", "opencode")
	if err := os.MkdirAll(cfg, 0o755); err != nil {
		t.Fatal(err)
	}

	got, err := (opencode{}).Install()
	if err != nil {
		t.Fatalf("Install() = %v", err)
	}
	want := filepath.Join(cfg, opencodeInstructionsFile)
	if got != want {
		t.Errorf("Install() path = %q, want %q", got, want)
	}

	body, err := os.ReadFile(want)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(body), sectionBeginPrefix+skills.Version+sectionBeginSuffix) {
		t.Errorf("AGENTS.md missing BEGIN marker:\n%s", body)
	}
	if !strings.Contains(string(body), sectionEndMarker) {
		t.Errorf("AGENTS.md missing END marker:\n%s", body)
	}
	if _, err := os.Stat(filepath.Join(cfg, opencodeRefsDir, "SKILL.md")); err != nil {
		t.Errorf("expected SKILL.md under refs root: %v", err)
	}
}

func TestOpenCode_Install_Idempotent(t *testing.T) {
	home := t.TempDir()
	withHomeDir(t, home)
	t.Setenv("XDG_CONFIG_HOME", "")
	if err := os.MkdirAll(filepath.Join(home, ".config", "opencode"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := (opencode{}).Install(); err != nil {
		t.Fatal(err)
	}
	first, err := os.ReadFile(filepath.Join(home, ".config", "opencode", opencodeInstructionsFile))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := (opencode{}).Install(); err != nil {
		t.Fatalf("second Install() = %v", err)
	}
	second, err := os.ReadFile(filepath.Join(home, ".config", "opencode", opencodeInstructionsFile))
	if err != nil {
		t.Fatal(err)
	}
	if string(first) != string(second) {
		t.Errorf("idempotence broken\n--- first ---\n%s\n--- second ---\n%s", first, second)
	}
}

func TestOpenCode_Scope_IsUser(t *testing.T) {
	if got := (opencode{}).Scope(); got != ScopeUser {
		t.Errorf("Scope() = %v, want ScopeUser", got)
	}
}

func TestRegistry_ContainsOpenCode(t *testing.T) {
	if Find("opencode") == nil {
		t.Fatal("opencode target not registered")
	}
}

func TestXDGConfigDir(t *testing.T) {
	home := t.TempDir()
	withHomeDir(t, home)

	t.Setenv("XDG_CONFIG_HOME", "")
	got, err := xdgConfigDir()
	if err != nil {
		t.Fatal(err)
	}
	if got != filepath.Join(home, ".config") {
		t.Errorf("default fallback = %q, want %q", got, filepath.Join(home, ".config"))
	}

	t.Setenv("XDG_CONFIG_HOME", "/some/path")
	got, err = xdgConfigDir()
	if err != nil {
		t.Fatal(err)
	}
	if got != "/some/path" {
		t.Errorf("XDG override = %q, want %q", got, "/some/path")
	}

	// Per the XDG Base Directory spec, a relative XDG_CONFIG_HOME must
	// be ignored — falling back to ~/.config rather than potentially
	// writing into the dev's cwd via a misconfigured env.
	t.Setenv("XDG_CONFIG_HOME", "relative/path")
	got, err = xdgConfigDir()
	if err != nil {
		t.Fatal(err)
	}
	if got != filepath.Join(home, ".config") {
		t.Errorf("relative XDG should fall back to ~/.config = %q, want %q", got, filepath.Join(home, ".config"))
	}
}
