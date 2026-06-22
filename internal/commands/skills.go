package commands

import (
	"fmt"

	"github.com/deployhq/deployhq-cli/internal/output"
	"github.com/deployhq/deployhq-cli/internal/skillinstaller"
	"github.com/spf13/cobra"
)

func newSkillsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "skills",
		Short: "Manage DeployHQ skill installs for local AI agents",
		Long: `Install the DeployHQ agent skill into AI coding tools on this machine
(Claude Code, Cursor, etc.). Run during 'dhq hello' or standalone.

Examples:
  dhq skills list                       Show detected agents and skill status
  dhq skills install                    Install for every detected agent
  dhq skills install --agent claude-code  Install for a single agent`,
	}
	cmd.AddCommand(newSkillsListCmd())
	cmd.AddCommand(newSkillsInstallCmd())
	return cmd
}

// skillRow is the rendered row type for `dhq skills list`.
type skillRow struct {
	Name        string `json:"name"`
	DisplayName string `json:"display_name"`
	Status      string `json:"status"`
}

func newSkillsListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List detected AI agents and skill install status",
		RunE: func(cmd *cobra.Command, args []string) error {
			env := cliCtx.Envelope

			rows := []skillRow{}
			for _, t := range skillinstaller.All() {
				rows = append(rows, skillRow{
					Name:        t.Name(),
					DisplayName: t.DisplayName(),
					Status:      t.Detect().String(),
				})
			}

			return env.WriteData(rows, []string{"NAME", "DISPLAY NAME", "STATUS"}, func(v interface{}) []string {
				r := v.(skillRow)
				return []string{r.Name, r.DisplayName, r.Status}
			})
		},
	}
}

func newSkillsInstallCmd() *cobra.Command {
	var agentFlag string
	cmd := &cobra.Command{
		Use:   "install",
		Short: "Install the DeployHQ skill for detected (or named) AI agents",
		RunE: func(cmd *cobra.Command, args []string) error {
			env := cliCtx.Envelope

			var targets []skillinstaller.Target
			var skippedProject []string
			if agentFlag != "" {
				t := skillinstaller.Find(agentFlag)
				if t == nil {
					return &output.UserError{
						Message: fmt.Sprintf("Unknown agent: %q", agentFlag),
						Hint:    "Run 'dhq skills list' to see supported agents.",
					}
				}
				targets = []skillinstaller.Target{t}
			} else {
				// Bulk install only touches user-scope targets — installing
				// project-scope ones (Copilot, Cline, Kiro, Antigravity)
				// would silently mutate the current repo, which we promise
				// not to do without --agent. Defer those to an explicit
				// `dhq skills install --agent <name>` invocation.
				for _, d := range skillinstaller.DetectInstalled() {
					if d.Target.Scope() == skillinstaller.ScopeUser {
						targets = append(targets, d.Target)
					} else {
						skippedProject = append(skippedProject, d.Target.Name())
					}
				}
			}

			if len(targets) == 0 {
				return &output.UserError{
					Message: "No supported AI agents detected on this machine",
					Hint:    "Install one (e.g. Claude Code) and re-run, or pass --agent <name>.",
				}
			}

			var failed int
			for _, t := range targets {
				path, err := t.Install()
				if err != nil {
					env.Warn("Could not install %s skill: %v", t.DisplayName(), err)
					failed++
					continue
				}
				output.ColorGreen.Fprintf(env.Stderr, "Installed %s skill → %s\n", t.DisplayName(), path) //nolint:errcheck
				if n, ok := t.(skillinstaller.Noter); ok {
					if note := n.PostInstallNote(); note != "" {
						env.Status("  %s", note)
					}
				}
			}

			if len(skippedProject) > 0 {
				env.Status("Skipped project-scope agents (modify the current repo, opt-in only):")
				for _, name := range skippedProject {
					env.Status("  - %s — install with: dhq skills install --agent %s", name, name)
				}
			}

			if failed > 0 {
				return &output.InternalError{Message: fmt.Sprintf("%d install(s) failed", failed)}
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&agentFlag, "agent", "", "Install for a specific agent only (e.g. claude-code)")
	return cmd
}
