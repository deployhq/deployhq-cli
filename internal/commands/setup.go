package commands

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/deployhq/deployhq-cli/internal/output"
	"github.com/spf13/cobra"
)

// agentSetup defines the configuration for an agent integration command.
type agentSetup struct {
	Use       string // cobra Use field (e.g. "claude")
	Short     string // one-line description
	Name      string // display name (e.g. "Claude Code")
	DotDir    string // directory name (e.g. ".claude")
	ExtraFile string // optional extra file to write (besides SKILL.md)
}

var agents = []agentSetup{
	{
		Use:       "claude",
		Short:     "Install Claude Code integration",
		Name:      "Claude Code",
		DotDir:    ".claude",
		ExtraFile: "deployhq-commands.md",
	},
	{
		Use:    "codex",
		Short:  "Install OpenAI Codex integration",
		Name:   "Codex",
		DotDir: ".codex",
	},
}

func newSetupCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "setup",
		Short: "Install agent plugins",
		Long:  "Install DeployHQ agent integration files for AI coding assistants.",
	}

	for _, a := range agents {
		cmd.AddCommand(newAgentSetupCmd(a))
	}

	return cmd
}

func newAgentSetupCmd(a agentSetup) *cobra.Command {
	var uninstall bool
	var project bool

	cmd := &cobra.Command{
		Use:   a.Use,
		Short: a.Short,
		Long: fmt.Sprintf(`Install %s agent integration files.

By default, files are installed in ~/%s/ (user-level) so the skill
is available in all sessions. Use --project to install in the current
directory's %s/ instead.`, a.Name, a.DotDir, a.DotDir),
		RunE: func(cmd *cobra.Command, args []string) error {
			env := cliCtx.Envelope

			scope := "user"
			dir := ""
			if project {
				dir = a.DotDir
				scope = "project"
			} else {
				home, err := os.UserHomeDir()
				if err != nil {
					return &output.InternalError{Message: "find home directory", Cause: err}
				}
				dir = filepath.Join(home, a.DotDir)
			}

			// Collect the files this agent installs
			files := installedFiles(a, dir)

			if uninstall {
				return removeSetupFiles(files, dir, a.Name)
			}

			if err := os.MkdirAll(dir, 0755); err != nil {
				return &output.InternalError{Message: fmt.Sprintf("create %s directory", a.DotDir), Cause: err}
			}

			// Write SKILL.md
			if err := os.WriteFile(files["SKILL.md"], []byte(skillMD), 0644); err != nil {
				return &output.InternalError{Message: "write SKILL.md", Cause: err}
			}

			// Write optional extra file
			if a.ExtraFile != "" {
				content := "# DeployHQ CLI Commands\n\nRun `dhq commands --json` to get the full command catalog.\n\nRun `dhq --help` for usage information.\n"
				if err := os.WriteFile(files[a.ExtraFile], []byte(content), 0644); err != nil {
					return &output.InternalError{Message: fmt.Sprintf("write %s", a.ExtraFile), Cause: err}
				}
			}

			env.Status("Installed %s integration (%s-level):", a.Name, scope)
			for _, p := range files {
				env.Status("  %s", p)
			}
			uninstallFlag := ""
			if project {
				uninstallFlag = " --project"
			}
			env.Status("\nTo uninstall: dhq setup %s --uninstall%s", a.Use, uninstallFlag)
			return nil
		},
	}

	cmd.Flags().BoolVar(&uninstall, "uninstall", false, "Remove installed files")
	cmd.Flags().BoolVar(&project, "project", false,
		fmt.Sprintf("Install to %s/ (project-level) instead of ~/%s/ (user-level)", a.DotDir, a.DotDir))
	return cmd
}

// installedFiles returns a map of logical name → file path for the files an agent installs.
func installedFiles(a agentSetup, dir string) map[string]string {
	files := map[string]string{
		"SKILL.md": filepath.Join(dir, "SKILL.md"),
	}
	if a.ExtraFile != "" {
		files[a.ExtraFile] = filepath.Join(dir, a.ExtraFile)
	}
	return files
}

// removeSetupFiles removes only the files we installed, not the whole directory
// (which may contain the user's own files like CLAUDE.md, memory, etc.).
func removeSetupFiles(files map[string]string, dir, name string) error {
	removed := 0
	for _, f := range files {
		if err := os.Remove(f); err == nil {
			removed++
		}
	}

	if removed == 0 {
		cliCtx.Envelope.Status("%s integration not found at %s", name, dir)
	} else {
		cliCtx.Envelope.Status("Removed %s integration from %s (%d files)", name, dir, removed)
	}
	return nil
}

// skillMD is the embedded SKILL.md content.
// It can also be fetched remotely from the DeployHQ docs.
const skillMD = `# DeployHQ CLI — Agent Skill Guide

## Identity
DeployHQ is a deployment automation platform. The ` + "`dhq`" + ` CLI manages projects, servers, and deployments.

## Authentication
` + "```" + `
# Environment variables (CI/agents — no login needed)
export DEPLOYHQ_API_KEY=your-api-key
export DEPLOYHQ_ACCOUNT=your-account
export DEPLOYHQ_EMAIL=your-email

# Or interactive login
dhq auth login
` + "```" + `

## Quick Reference

### Discovery
` + "```" + `
dhq commands --json          # Full command catalog
dhq --help                   # Usage overview
dhq <command> --help         # Command details
` + "```" + `

### Common Workflows

#### Deploy and wait for completion
` + "```" + `
dhq deploy -p <project> -s <server> --wait --json
` + "```" + `

#### Deploy latest revision
` + "```" + `
dhq deploy -p <project> -s <server> --use-latest --wait --json
` + "```" + `

#### Check deployment status
` + "```" + `
dhq deployments show <id> -p <project> --json
` + "```" + `

#### Watch a deployment in real-time
` + "```" + `
dhq deployments watch <id> -p <project>
` + "```" + `

#### View deployment logs
` + "```" + `
dhq deployments logs <id> -p <project>
` + "```" + `

#### Rollback
` + "```" + `
dhq rollback <deployment-id> -p <project> --json
` + "```" + `

#### Manage environment variables
` + "```" + `
dhq env-vars list -p <project> --json
dhq env-vars create --name KEY --value VALUE --locked -p <project>
dhq global-env-vars create --name KEY --value VALUE
` + "```" + `

#### Test repository and server connectivity
` + "```" + `
dhq test-access -p <project> --json
dhq test-access -p <project> --server <server> --json
` + "```" + `

#### Escape hatch (any API endpoint)
` + "```" + `
dhq api GET /projects/<id>/environment_variables
dhq api POST /projects/<id>/config_files --body '{"config_file":{...}}'
` + "```" + `

## Decision Tree

### "Deploy code"
1. ` + "`dhq projects list --json`" + ` → find project
2. ` + "`dhq servers list -p <project> --json`" + ` → find server
3. ` + "`dhq deploy -p <project> -s <server> --wait --json`" + ` → deploy and wait
4. On failure: ` + "`dhq deployments logs <id> -p <project>`" + ` → read logs

### "Check what's deployed"
1. ` + "`dhq deployments list -p <project> --json`" + ` → recent deployments
2. ` + "`dhq deployments show <id> -p <project> --json`" + ` → details + steps

### "Something went wrong"
1. ` + "`dhq deployments logs <id> -p <project>`" + ` → read logs
2. ` + "`dhq rollback <id> -p <project> --json`" + ` → rollback if needed
3. ` + "`dhq deployments abort <id> -p <project>`" + ` → abort if running

### "Test connectivity"
1. ` + "`dhq test-access -p <project> --json`" + ` → test all servers + repo
2. ` + "`dhq test-access -p <project> --server <name> --json`" + ` → test single server

### "Manage project config"
1. ` + "`dhq env-vars list -p <project> --json`" + ` → environment variables
2. ` + "`dhq config-files list -p <project> --json`" + ` → config files
3. ` + "`dhq build-commands list -p <project> --json`" + ` → build pipeline
4. ` + "`dhq ssh-commands list -p <project> --json`" + ` → SSH commands
5. ` + "`dhq servers list -p <project> --json`" + ` → servers

## Invariants
- Always use ` + "`--json`" + ` for machine-readable output
- JSON responses include ` + "`breadcrumbs`" + ` with suggested next commands
- Exit code 0 = success, 1 = failure (check JSON ` + "`ok`" + ` field for details)
- Empty results return exit 0 with empty ` + "`data`" + ` (not an error)
- ` + "`dhq api`" + ` covers any endpoint not in the command tree
- Use ` + "`--wait`" + ` on deploy to block until completion (exits non-zero on failure)

## Triggers
- User mentions "deploy", "deployment", "release" → deployment workflow
- User mentions "server", "hosting" → server management
- User mentions "rollback", "revert" → rollback workflow
- User mentions "env var", "environment", "config" → configuration management
- User mentions "test", "connectivity", "access" → test-access workflow
- User mentions "DeployHQ", "deployhq" → general CLI usage
`
