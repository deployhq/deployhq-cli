package commands

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/deployhq/deployhq-cli/internal/harness"
	"github.com/deployhq/deployhq-cli/internal/output"
	"github.com/deployhq/deployhq-cli/internal/skillinstaller"
)

// fakeTarget is a minimal Target the hello-flow tests can fully control.
// It records whether Install was called so each test can assert exactly
// which targets were touched.
type fakeTarget struct {
	name        string
	displayName string
	scope       skillinstaller.Scope
	status      skillinstaller.Status
	installErr  error
	installPath string
	installed   bool
}

func (f *fakeTarget) Name() string                  { return f.name }
func (f *fakeTarget) DisplayName() string           { return f.displayName }
func (f *fakeTarget) Scope() skillinstaller.Scope   { return f.scope }
func (f *fakeTarget) Detect() skillinstaller.Status { return f.status }
func (f *fakeTarget) Install() (string, error) {
	f.installed = true
	if f.installErr != nil {
		return "", f.installErr
	}
	return f.installPath, nil
}

// withFakeDeps swaps the three test seams in hello_skills.go for the
// duration of the test. Pass runtimeName="" to simulate "no agent
// detected at runtime". confirmAnswer is only consulted when the prompt
// path is reached.
func withFakeDeps(t *testing.T, detected []skillinstaller.DetectResult, runtimeName string, confirmAnswer bool) {
	t.Helper()
	origDetect := detectInstalledFn
	origRuntime := detectRuntimeAgentFn
	origConfirm := confirmInstallFn

	detectInstalledFn = func() []skillinstaller.DetectResult { return detected }
	detectRuntimeAgentFn = func() harness.AgentInfo {
		return harness.AgentInfo{Detected: runtimeName != "", Name: runtimeName}
	}
	confirmInstallFn = func(string) bool { return confirmAnswer }

	t.Cleanup(func() {
		detectInstalledFn = origDetect
		detectRuntimeAgentFn = origRuntime
		confirmInstallFn = origConfirm
	})
}

// newTestEnv returns an Envelope writing to the returned buffer.
// NonInteractive is configurable via the second arg.
func newTestEnv(nonInteractive bool) (*output.Envelope, *bytes.Buffer) {
	var buf bytes.Buffer
	env := &output.Envelope{
		Stdout:         &buf,
		Stderr:         &buf,
		Logger:         output.NewLogger(),
		NonInteractive: nonInteractive,
	}
	return env, &buf
}

func TestOfferSkillInstall_NoDetected_NoOp(t *testing.T) {
	withFakeDeps(t, nil, "", true)
	env, buf := newTestEnv(false)

	offerSkillInstall(env)

	if buf.Len() != 0 {
		t.Errorf("expected no output for empty detection, got: %q", buf.String())
	}
}

func TestOfferSkillInstall_RuntimeAgent_AutoInstallsWithoutPrompt(t *testing.T) {
	runtime := &fakeTarget{name: "claude-code", displayName: "Claude Code", scope: skillinstaller.ScopeUser, status: skillinstaller.StatusAvailable, installPath: "/fake/claude"}
	other := &fakeTarget{name: "cursor", displayName: "Cursor", scope: skillinstaller.ScopeUser, status: skillinstaller.StatusAvailable, installPath: "/fake/cursor"}

	confirmCalled := false
	withFakeDeps(t, []skillinstaller.DetectResult{
		{Target: runtime, Status: runtime.status},
		{Target: other, Status: other.status},
	}, "claude-code", false)
	confirmInstallFn = func(string) bool {
		confirmCalled = true
		return false
	}
	env, buf := newTestEnv(false)

	offerSkillInstall(env)

	if !runtime.installed {
		t.Error("runtime agent (claude-code) was not auto-installed")
	}
	if !confirmCalled {
		t.Error("expected confirmation prompt for the non-runtime agent")
	}
	if other.installed {
		t.Error("non-runtime agent should not be installed after user declines")
	}
	// Sanity: status output mentions the runtime agent line.
	if !strings.Contains(buf.String(), "Claude Code") {
		t.Errorf("expected runtime install status in output: %q", buf.String())
	}
}

func TestOfferSkillInstall_ProjectScopeTargets_AreSkipped(t *testing.T) {
	// Only project-scope targets detected. None should install — those are
	// reserved for explicit `dhq skills install --agent <name>`.
	copilot := &fakeTarget{name: "copilot", displayName: "GitHub Copilot", scope: skillinstaller.ScopeProject, status: skillinstaller.StatusAvailable}
	cline := &fakeTarget{name: "cline", displayName: "Cline", scope: skillinstaller.ScopeProject, status: skillinstaller.StatusAvailable}

	confirmCalled := false
	withFakeDeps(t, []skillinstaller.DetectResult{
		{Target: copilot, Status: copilot.status},
		{Target: cline, Status: cline.status},
	}, "", false)
	confirmInstallFn = func(string) bool {
		confirmCalled = true
		return true
	}
	env, _ := newTestEnv(false)

	offerSkillInstall(env)

	if copilot.installed {
		t.Error("Copilot (project-scope) should not be installed from hello hook")
	}
	if cline.installed {
		t.Error("Cline (project-scope) should not be installed from hello hook")
	}
	if confirmCalled {
		t.Error("no prompt should appear when only project-scope targets are detected")
	}
}

func TestOfferSkillInstall_NonInteractive_InstallsRuntime_SkipsPrompt(t *testing.T) {
	runtime := &fakeTarget{name: "claude-code", displayName: "Claude Code", scope: skillinstaller.ScopeUser, status: skillinstaller.StatusAvailable, installPath: "/fake/claude"}
	other := &fakeTarget{name: "cursor", displayName: "Cursor", scope: skillinstaller.ScopeUser, status: skillinstaller.StatusAvailable, installPath: "/fake/cursor"}

	confirmCalled := false
	withFakeDeps(t, []skillinstaller.DetectResult{
		{Target: runtime, Status: runtime.status},
		{Target: other, Status: other.status},
	}, "claude-code", true)
	confirmInstallFn = func(string) bool {
		confirmCalled = true
		return true
	}
	env, _ := newTestEnv(true) // NonInteractive

	offerSkillInstall(env)

	if !runtime.installed {
		t.Error("runtime auto-install should still happen in non-interactive mode")
	}
	if other.installed {
		t.Error("non-runtime install must not happen in non-interactive mode")
	}
	if confirmCalled {
		t.Error("no confirm prompt allowed in non-interactive mode")
	}
}

func TestOfferSkillInstall_PromptYes_InstallsOthers(t *testing.T) {
	a := &fakeTarget{name: "cursor", displayName: "Cursor", scope: skillinstaller.ScopeUser, status: skillinstaller.StatusAvailable, installPath: "/fake/cursor"}
	b := &fakeTarget{name: "windsurf", displayName: "Windsurf", scope: skillinstaller.ScopeUser, status: skillinstaller.StatusAvailable, installPath: "/fake/windsurf"}

	withFakeDeps(t, []skillinstaller.DetectResult{
		{Target: a, Status: a.status},
		{Target: b, Status: b.status},
	}, "", true) // no runtime, confirm: yes
	env, _ := newTestEnv(false)

	offerSkillInstall(env)

	if !a.installed || !b.installed {
		t.Errorf("expected both targets installed after user confirmed; a=%v b=%v", a.installed, b.installed)
	}
}

func TestOfferSkillInstall_PromptNo_NoInstalls(t *testing.T) {
	a := &fakeTarget{name: "cursor", displayName: "Cursor", scope: skillinstaller.ScopeUser, status: skillinstaller.StatusAvailable, installPath: "/fake/cursor"}

	withFakeDeps(t, []skillinstaller.DetectResult{
		{Target: a, Status: a.status},
	}, "", false) // no runtime, confirm: no
	env, buf := newTestEnv(false)

	offerSkillInstall(env)

	if a.installed {
		t.Error("target installed despite user declining the prompt")
	}
	if !strings.Contains(buf.String(), "Skipping") {
		t.Errorf("expected a 'Skipping' message after decline: %q", buf.String())
	}
}

func TestOfferSkillInstall_AlreadyInstalled_NotReinstalled(t *testing.T) {
	// StatusInstalled means Needed() returns false; we must not Install
	// again on every `dhq hello`.
	t1 := &fakeTarget{name: "cursor", displayName: "Cursor", scope: skillinstaller.ScopeUser, status: skillinstaller.StatusInstalled, installPath: "/fake/cursor"}

	confirmCalled := false
	withFakeDeps(t, []skillinstaller.DetectResult{
		{Target: t1, Status: t1.status},
	}, "", true)
	confirmInstallFn = func(string) bool {
		confirmCalled = true
		return true
	}
	env, _ := newTestEnv(false)

	offerSkillInstall(env)

	if t1.installed {
		t.Error("already-installed target was re-installed (Needed() should have filtered it)")
	}
	if confirmCalled {
		t.Error("no prompt should appear when the only target is already installed")
	}
}

func TestOfferSkillInstall_InstallError_IsNonFatal(t *testing.T) {
	// Install errors are surfaced via env.Warn but must not panic or
	// halt processing of the remaining targets.
	failing := &fakeTarget{name: "cursor", displayName: "Cursor", scope: skillinstaller.ScopeUser, status: skillinstaller.StatusAvailable, installErr: errors.New("disk full")}
	ok := &fakeTarget{name: "windsurf", displayName: "Windsurf", scope: skillinstaller.ScopeUser, status: skillinstaller.StatusAvailable, installPath: "/fake/windsurf"}

	withFakeDeps(t, []skillinstaller.DetectResult{
		{Target: failing, Status: failing.status},
		{Target: ok, Status: ok.status},
	}, "", true)
	env, buf := newTestEnv(false)

	offerSkillInstall(env)

	if !ok.installed {
		t.Error("second target should have installed even after the first one errored")
	}
	if !strings.Contains(buf.String(), "Warning") || !strings.Contains(buf.String(), "Cursor") {
		t.Errorf("expected a warning about the failing Cursor install: %q", buf.String())
	}
}
