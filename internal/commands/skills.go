package commands

import (
	"fmt"
	"io/fs"
	"sort"
	"strings"

	"github.com/deployhq/deployhq-cli/internal/output"
	"github.com/deployhq/deployhq-cli/internal/skillinstaller"
	"github.com/deployhq/deployhq-cli/skills"
	"github.com/spf13/cobra"
)

func newSkillsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "skills",
		Short: "Manage DeployHQ skill installs for local AI agents",
		Long: `Install the DeployHQ agent skill into AI coding tools on this machine
(Claude Code, Cursor, etc.). Run during 'dhq hello' or standalone.

Examples:
  dhq skills show                         Print the skill, to read it right away
  dhq skills list                         Show every supported agent and status
  dhq skills install                      Install for detected user-scope agents
                                          (Claude Code, Cursor, etc.). Skips
                                          project-scope agents to avoid silently
                                          mutating the current repo.
  dhq skills install --agent copilot      Install for a project-scope agent
                                          (writes into the current repo)
  dhq skills install --agent claude-code  Install for any specific agent`,
	}
	cmd.AddCommand(newSkillsShowCmd())
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
		Short: "List supported AI agents and their skill install status",
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
				if len(skippedProject) > 0 {
					// Detected agents exist, they're just project-scope —
					// hide them from the bare command to avoid silently
					// writing into the user's repo. Tell the user exactly
					// which ones can be opted in.
					hint := "Available project-scope agents (run with --agent to install):\n"
					for _, name := range skippedProject {
						hint += fmt.Sprintf("  - dhq skills install --agent %s\n", name)
					}
					return &output.UserError{
						Message: "No user-scope AI agents available for bulk install",
						Hint:    hint,
					}
				}
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

// skillDir is the embedded skill's root in skills.FS.
const skillDir = "deployhq"

// skillReferences returns the names of the skill's reference docs (without
// ".md"), sorted.
func skillReferences() []string {
	entries, err := fs.ReadDir(skills.FS, skillDir+"/references")
	if err != nil {
		return nil
	}
	var names []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".md") {
			names = append(names, strings.TrimSuffix(e.Name(), ".md"))
		}
	}
	sort.Strings(names)
	return names
}

// skillShowOutput is the --json shape of 'dhq skills show'.
type skillShowOutput struct {
	Name       string   `json:"name"`
	Content    string   `json:"content"`
	References []string `json:"references"`
}

// newSkillsShowCmd prints the embedded skill. A skill installed with
// 'dhq skills install' is only loaded by the agent's next session, so an
// agent that is setting DeployHQ up right now reads it this way instead.
func newSkillsShowCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "show [reference]",
		Short: "Print the DeployHQ skill, so an AI agent can read it right away",
		Long: `Print DeployHQ's skill for AI agents: the setup steps, including when to ask
the person. Nothing is installed or changed.

An agent can read it straight away, in the session it is working in. A skill
installed with 'dhq skills install' is only loaded by the agent's next session.

With a reference name, print that reference document instead. The names are
listed at the end of the skill.`,
		Example: `  # Read the skill
  dhq skills show

  # Read one of its references
  dhq skills show launch`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			env := cliCtx.Envelope
			refs := skillReferences()

			name, path := "SKILL", skillDir+"/SKILL.md"
			if len(args) == 1 {
				name = strings.TrimSuffix(args[0], ".md")
				path = skillDir + "/references/" + name + ".md"
			}
			data, err := skills.FS.ReadFile(path)
			if err != nil {
				return &output.UserError{
					Message: fmt.Sprintf("Unknown skill reference: %q", name),
					Hint:    "References: " + strings.Join(refs, ", "),
				}
			}

			if env.JSONMode {
				return env.WriteJSON(output.NewResponse(skillShowOutput{Name: name, Content: string(data), References: refs},
					fmt.Sprintf("DeployHQ skill: %s", name)))
			}
			// Plain Markdown even when piped: this is a document to read.
			fmt.Fprint(env.Stdout, string(data)) //nolint:errcheck
			if len(args) == 0 {
				fmt.Fprintf(env.Stdout, "\n---\nThe references linked above are printed with 'dhq skills show <name>': %s\n", strings.Join(refs, ", ")) //nolint:errcheck
			}
			return nil
		},
	}
}
