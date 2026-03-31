package commands

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/deployhq/deployhq-cli/internal/output"
	"github.com/spf13/cobra"
)

func newSetupCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "setup",
		Short: "Install agent plugins",
		Long:  "Install DeployHQ agent integration files for AI coding assistants.",
	}

	cmd.AddCommand(
		newSetupClaudeCmd(),
		newSetupCodexCmd(),
	)

	return cmd
}

func newSetupClaudeCmd() *cobra.Command {
	var uninstall bool

	cmd := &cobra.Command{
		Use:   "claude",
		Short: "Install Claude Code integration",
		Long:  "Install .claude/ plugin files for Claude Code agent integration.",
		RunE: func(cmd *cobra.Command, args []string) error {
			env := cliCtx.Envelope
			dir := ".claude"

			if uninstall {
				return removeSetup(dir, "Claude Code")
			}

			if err := os.MkdirAll(dir, 0755); err != nil {
				return &output.InternalError{Message: "create .claude directory", Cause: err}
			}

			// Write SKILL.md
			skillPath := filepath.Join(dir, "SKILL.md")
			if err := os.WriteFile(skillPath, []byte(skillMD), 0644); err != nil {
				return &output.InternalError{Message: "write SKILL.md", Cause: err}
			}

			// Write commands.json reference
			commandsRef := filepath.Join(dir, "deployhq-commands.md")
			content := "# DeployHQ CLI Commands\n\nRun `dhq commands --json` to get the full command catalog.\n\nRun `dhq --help` for usage information.\n"
			if err := os.WriteFile(commandsRef, []byte(content), 0644); err != nil {
				return &output.InternalError{Message: "write commands reference", Cause: err}
			}

			env.Status("Installed Claude Code integration:")
			env.Status("  %s (agent workflow guide)", skillPath)
			env.Status("  %s (commands reference)", commandsRef)
			env.Status("\nTo uninstall: dhq setup claude --uninstall")
			return nil
		},
	}

	cmd.Flags().BoolVar(&uninstall, "uninstall", false, "Remove installed files")
	return cmd
}

func newSetupCodexCmd() *cobra.Command {
	var uninstall bool

	cmd := &cobra.Command{
		Use:   "codex",
		Short: "Install OpenAI Codex integration",
		Long:  "Install codex agent integration files.",
		RunE: func(cmd *cobra.Command, args []string) error {
			env := cliCtx.Envelope
			dir := ".codex"

			if uninstall {
				return removeSetup(dir, "Codex")
			}

			if err := os.MkdirAll(dir, 0755); err != nil {
				return &output.InternalError{Message: "create .codex directory", Cause: err}
			}

			// Write SKILL.md
			skillPath := filepath.Join(dir, "SKILL.md")
			if err := os.WriteFile(skillPath, []byte(skillMD), 0644); err != nil {
				return &output.InternalError{Message: "write SKILL.md", Cause: err}
			}

			env.Status("Installed Codex integration:")
			env.Status("  %s (agent workflow guide)", skillPath)
			env.Status("\nTo uninstall: dhq setup codex --uninstall")
			return nil
		},
	}

	cmd.Flags().BoolVar(&uninstall, "uninstall", false, "Remove installed files")
	return cmd
}

func removeSetup(dir, name string) error {
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		cliCtx.Envelope.Status("%s integration not found at %s", name, dir)
		return nil
	}

	if err := os.RemoveAll(dir); err != nil {
		return &output.InternalError{Message: fmt.Sprintf("remove %s directory", dir), Cause: err}
	}

	cliCtx.Envelope.Status("Removed %s integration from %s", name, dir)
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
- User mentions "DeployHQ", "deployhq" → general CLI usage
`
