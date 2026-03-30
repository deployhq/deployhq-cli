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
			content := "# DeployHQ CLI Commands\n\nRun `deployhq commands --json` to get the full command catalog.\n\nRun `deployhq --help` for usage information.\n"
			if err := os.WriteFile(commandsRef, []byte(content), 0644); err != nil {
				return &output.InternalError{Message: "write commands reference", Cause: err}
			}

			env.Status("Installed Claude Code integration:")
			env.Status("  %s (agent workflow guide)", skillPath)
			env.Status("  %s (commands reference)", commandsRef)
			env.Status("\nTo uninstall: deployhq setup claude --uninstall")
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
			env.Status("\nTo uninstall: deployhq setup codex --uninstall")
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
DeployHQ is a deployment automation platform. The ` + "`deployhq`" + ` CLI manages projects, servers, and deployments.

## Authentication
Set ` + "`DEPLOYHQ_API_TOKEN`" + ` environment variable, or run ` + "`deployhq auth login`" + `.

## Quick Reference

### Discovery
` + "```" + `
deployhq commands --json          # Full command catalog
deployhq --help                   # Usage overview
deployhq <command> --help         # Command details
` + "```" + `

### Common Workflows

#### List projects
` + "```" + `
deployhq projects list --json
` + "```" + `

#### Deploy latest to a server
` + "```" + `
deployhq deploy -p <project> -s <server> --use-latest --json
` + "```" + `

#### Check deployment status
` + "```" + `
deployhq deployments show <id> -p <project> --json
` + "```" + `

#### View deployment logs
` + "```" + `
deployhq deployments logs <id> -p <project>
` + "```" + `

#### Rollback
` + "```" + `
deployhq rollback <deployment-id> -p <project> --json
` + "```" + `

#### Escape hatch (any API endpoint)
` + "```" + `
deployhq api GET /projects/<id>/environment_variables
deployhq api POST /projects/<id>/config_files --body '{"config_file":{...}}'
` + "```" + `

## Decision Tree

### "Deploy code"
1. ` + "`deployhq projects list --json`" + ` → find project
2. ` + "`deployhq servers list -p <project> --json`" + ` → find server
3. ` + "`deployhq deploy -p <project> -s <server> --json`" + ` → create deployment
4. ` + "`deployhq deployments show <id> -p <project> --json`" + ` → check status

### "Check what's deployed"
1. ` + "`deployhq deployments list -p <project> --json`" + ` → recent deployments
2. ` + "`deployhq deployments show <id> -p <project> --json`" + ` → details + steps

### "Something went wrong"
1. ` + "`deployhq deployments logs <id> -p <project>`" + ` → read logs
2. ` + "`deployhq rollback <id> -p <project> --json`" + ` → rollback if needed
3. ` + "`deployhq deployments abort <id> -p <project>`" + ` → abort if running

## Invariants
- Always use ` + "`--json`" + ` for machine-readable output
- JSON responses include ` + "`breadcrumbs`" + ` with suggested next commands
- Exit code 0 = success, 1 = failure (check JSON ` + "`ok`" + ` field for details)
- Empty results return exit 0 with empty ` + "`data`" + ` (not an error)
- ` + "`deployhq api`" + ` covers any endpoint not in the command tree

## Triggers
- User mentions "deploy", "deployment", "release" → deployment workflow
- User mentions "server", "hosting" → server management
- User mentions "rollback", "revert" → rollback workflow
- User mentions "DeployHQ", "deployhq" → general CLI usage
`
