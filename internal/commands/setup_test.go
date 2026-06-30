package commands

import (
	"bytes"
	"io"
	"strings"
	"testing"

	"github.com/deployhq/deployhq-cli/internal/cli"
	"github.com/deployhq/deployhq-cli/internal/output"
	"github.com/deployhq/deployhq-cli/internal/skillinstaller"
)

// TestSetupSkillsNamesResolveToTargets locks the setup→skills agent-name
// mapping: every SkillsName the deprecation note points at must resolve to a
// real registered skills target, or the migration hint sends users to a
// nonexistent --agent value.
func TestSetupSkillsNamesResolveToTargets(t *testing.T) {
	for _, a := range agents {
		if a.SkillsName == "" {
			t.Errorf("agent %q has no SkillsName for the deprecation hint", a.Use)
			continue
		}
		if skillinstaller.Find(a.SkillsName) == nil {
			t.Errorf("agent %q maps to SkillsName %q, which is not a registered 'dhq skills' target",
				a.Use, a.SkillsName)
		}
	}
}

// TestSetupDeprecationNote verifies the note is mode-aware: it must not tell
// uninstall users to run an install command, and must not promise scope
// equivalence on the --project path.
func TestSetupDeprecationNote(t *testing.T) {
	a := agentSetup{Use: "claude", SkillsName: "claude-code"}

	t.Run("install", func(t *testing.T) {
		note := setupDeprecationNote(a, false, false)
		mustContain(t, note, "deprecated")
		mustContain(t, note, "dhq skills install --agent claude-code")
	})

	t.Run("project", func(t *testing.T) {
		note := setupDeprecationNote(a, false, true)
		mustContain(t, note, "claude-code")
		// Must flag that skills uses its own default scope, not project-local.
		mustContain(t, note, "default scope")
	})

	t.Run("uninstall", func(t *testing.T) {
		note := setupDeprecationNote(a, true, false)
		mustContain(t, note, "no uninstall")
		mustContain(t, note, "dhq setup claude --uninstall")
		// Must NOT point uninstall users at an install command.
		if strings.Contains(note, "install --agent") {
			t.Errorf("uninstall note should not suggest an install command: %q", note)
		}
	})
}

// TestSetupCommand_WarnsOnStderrNotStdout drives a real setup subcommand and
// asserts the deprecation warning lands on stderr while stdout stays clean —
// the load-bearing output-contract property of the deprecation.
func TestSetupCommand_WarnsOnStderrNotStdout(t *testing.T) {
	t.Setenv("HOME", t.TempDir()) // isolate from the real ~/.claude

	var stdout, stderr bytes.Buffer
	origCtx := cliCtx
	t.Cleanup(func() { cliCtx = origCtx })
	cliCtx = &cli.Context{
		Envelope: &output.Envelope{Stdout: &stdout, Stderr: &stderr, Logger: output.NewLogger()},
	}

	root := newSetupCmd()
	// --uninstall on a fresh HOME removes nothing (no files written), so this
	// exercises the warning path without filesystem side effects.
	root.SetArgs([]string{"claude", "--uninstall"})
	root.SetOut(io.Discard)
	root.SetErr(io.Discard)
	if err := root.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}

	if !strings.Contains(stderr.String(), "deprecated") {
		t.Errorf("deprecation warning missing from stderr: %q", stderr.String())
	}
	if strings.TrimSpace(stdout.String()) != "" {
		t.Errorf("stdout must stay clean (data channel), got: %q", stdout.String())
	}
}

func mustContain(t *testing.T, s, sub string) {
	t.Helper()
	if !strings.Contains(s, sub) {
		t.Errorf("expected %q to contain %q", s, sub)
	}
}
