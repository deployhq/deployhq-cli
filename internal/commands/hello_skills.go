package commands

import (
	"fmt"
	"strings"

	"github.com/deployhq/deployhq-cli/internal/harness"
	"github.com/deployhq/deployhq-cli/internal/output"
	"github.com/deployhq/deployhq-cli/internal/skillinstaller"
	"github.com/manifoldco/promptui"
)

// offerSkillInstall is the Wrangler-style post-login hook that detects locally
// installed AI agents and offers to install the DeployHQ skill for them.
//
// Behaviour:
//   - Runtime agent (the one currently running dhq, per harness.Detect) is
//     auto-installed without prompting when an install is Needed — if the
//     user is using dhq from inside Claude Code right now, they want it.
//   - Other agents detected on disk are batched into a single Y/n prompt.
//   - Errors are non-fatal: hello succeeds even if installs fail; users can
//     re-run `dhq skills install` later.
//
// The function is a no-op when nothing is detected, when nothing needs
// installing, or when env.NonInteractive is set.
func offerSkillInstall(env *output.Envelope) {
	detected := skillinstaller.DetectInstalled()
	if len(detected) == 0 {
		return
	}

	runtimeName := harness.Detect().Name

	var runtime *skillinstaller.DetectResult
	var others []skillinstaller.DetectResult
	for i, d := range detected {
		if !skillinstaller.Needed(d.Status) {
			continue
		}
		// Project-scope targets (e.g. Copilot's .github/copilot-instructions.md)
		// modify the current repo. Never install those as a side effect of
		// 'dhq hello' — they're opt-in via 'dhq skills install --agent <name>'.
		if d.Target.Scope() != skillinstaller.ScopeUser {
			continue
		}
		if d.Target.Name() == runtimeName {
			runtime = &detected[i]
			continue
		}
		others = append(others, d)
	}

	if runtime != nil {
		installOne(env, runtime.Target, "Installing DeployHQ skill for %s (you're using it now)…")
	}

	if len(others) == 0 || env.NonInteractive {
		return
	}

	names := make([]string, len(others))
	for i, d := range others {
		names[i] = d.Target.DisplayName()
	}
	label := fmt.Sprintf("Detected AI agents that could use the DeployHQ skill: %s.\n  Install for them now?", strings.Join(names, ", "))

	prompt := promptui.Prompt{
		Label:     label,
		IsConfirm: true,
		Default:   "Y",
	}
	if _, err := prompt.Run(); err != nil {
		// User said no, or aborted — fine.
		env.Status("Skipping. Run `dhq skills install` later if you change your mind.")
		return
	}

	for _, d := range others {
		installOne(env, d.Target, "Installing DeployHQ skill for %s…")
	}
}

// installOne runs Install on a single target and prints a result line.
// statusFmt receives the DisplayName via %s.
func installOne(env *output.Envelope, t skillinstaller.Target, statusFmt string) {
	env.Status(statusFmt, t.DisplayName())
	path, err := t.Install()
	if err != nil {
		env.Warn("Could not install %s skill: %v", t.DisplayName(), err)
		return
	}
	output.ColorGreen.Fprintf(env.Stderr, "  Installed %s skill → %s\n", t.DisplayName(), path) //nolint:errcheck
	if n, ok := t.(skillinstaller.Noter); ok {
		if note := n.PostInstallNote(); note != "" {
			env.Status("  %s", note)
		}
	}
}
