package commands

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/deployhq/deployhq-cli/internal/output"
	"github.com/spf13/cobra"
)

// blockBegin / blockEnd delimit the section managed by `dhq setup`
// inside files that may also contain unrelated user content (AGENTS.md, global_rules.md).
const (
	blockBegin = "<!-- BEGIN dhq -->"
	blockEnd   = "<!-- END dhq -->"
)

// errUnsupportedScope is returned when an agent does not support a given scope.
var errUnsupportedScope = errors.New("scope not supported")

type scope string

const (
	scopeUser    scope = "user"
	scopeProject scope = "project"
)

// writeStrategy controls how content is combined with the target file.
type writeStrategy int

const (
	// strategyOverwrite writes the entire file. Used when we own a dedicated path
	// (e.g. ~/.claude/skills/deployhq/SKILL.md, .cursor/rules/deployhq.mdc).
	strategyOverwrite writeStrategy = iota
	// strategyMarkedBlock inserts or replaces our delimited block within a shared
	// file (AGENTS.md, global_rules.md). On uninstall only the block is removed.
	strategyMarkedBlock
)

// agentSetup describes a single agent integration target.
type agentSetup struct {
	Use         string
	Short       string
	Name        string
	SkillsName  string // equivalent target name for `dhq skills install --agent`
	PathFor     func(scope) (string, error)
	Content     func() []byte
	StrategyFor func(scope) writeStrategy
}

func always(s writeStrategy) func(scope) writeStrategy {
	return func(scope) writeStrategy { return s }
}

var agents = []agentSetup{
	{
		Use:         "claude",
		Short:       "Install Claude Code skill",
		Name:        "Claude Code",
		SkillsName:  "claude-code",
		PathFor:     pathClaude,
		Content:     func() []byte { return []byte(skillFrontmatter + skillBody) },
		StrategyFor: always(strategyOverwrite),
	},
	{
		Use:         "codex",
		Short:       "Install OpenAI Codex AGENTS.md section",
		Name:        "Codex",
		SkillsName:  "codex",
		PathFor:     pathCodex,
		Content:     func() []byte { return []byte(skillBody) },
		StrategyFor: always(strategyMarkedBlock),
	},
	{
		Use:         "cursor",
		Short:       "Install Cursor project rule",
		Name:        "Cursor",
		SkillsName:  "cursor",
		PathFor:     pathCursor,
		Content:     func() []byte { return []byte(cursorFrontmatter + skillBody) },
		StrategyFor: always(strategyOverwrite),
	},
	{
		Use:        "windsurf",
		Short:      "Install Windsurf integration",
		Name:       "Windsurf",
		SkillsName: "windsurf",
		PathFor:    pathWindsurf,
		Content: func() []byte { return []byte(skillBody) },
		// User-level writes into the shared ~/.codeium/.../global_rules.md, so we
		// must merge with a marker block. Project-level writes a dedicated file we own.
		StrategyFor: func(sc scope) writeStrategy {
			if sc == scopeUser {
				return strategyMarkedBlock
			}
			return strategyOverwrite
		},
	},
}

func newSetupCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "setup",
		Short: "Install agent plugins (deprecated — use 'dhq skills')",
		Long: "Install DeployHQ agent integration files for AI coding assistants.\n\n" +
			"DEPRECATED: 'dhq setup' is superseded by 'dhq skills install', which\n" +
			"auto-detects installed agents and supports 12 of them (vs the 4 here).\n" +
			"This command still works but will be removed in a future release.\n" +
			"Migrate with 'dhq skills install' (or 'dhq skills install --agent <name>').",
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
		Long:  longHelp(a),
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			env := cliCtx.Envelope
			// 'dhq setup' is deprecated in favour of 'dhq skills'. Warn on every
			// use (stderr, so JSON/data on stdout is unaffected). The note is
			// mode-aware: 'dhq skills' has no uninstall and installs at each
			// agent's own default scope, so we don't claim a like-for-like
			// replacement for the --uninstall or --project paths.
			env.Warn("%s", setupDeprecationNote(a, uninstall, project))

			sc := scopeUser
			if project {
				sc = scopeProject
			}

			path, err := a.PathFor(sc)
			if err != nil {
				if errors.Is(err, errUnsupportedScope) {
					return &output.UserError{
						Message: fmt.Sprintf("%s does not support %s-level install; rerun with --project", a.Name, sc),
					}
				}
				return &output.InternalError{Message: "resolve install path", Cause: err}
			}

			strategy := a.StrategyFor(sc)
			if uninstall {
				return runUninstall(env, a, path, strategy)
			}
			return runInstall(env, a, path, sc, project, strategy)
		},
	}

	cmd.Flags().BoolVar(&uninstall, "uninstall", false, "Remove installed files")
	cmd.Flags().BoolVar(&project, "project", false,
		"Install at project level (current directory) instead of user-global")
	return cmd
}

// setupDeprecationNote builds the per-run deprecation warning for `dhq setup`.
// It is mode-aware because the successor command isn't a like-for-like drop-in:
//   - `dhq skills` has no uninstall, so the --uninstall path keeps using setup;
//   - `dhq skills install --agent <name>` installs at that agent's own default
//     scope (e.g. Cursor is user-scope there, project-scope here), so we don't
//     promise scope equivalence on the --project path.
func setupDeprecationNote(a agentSetup, uninstall, project bool) string {
	switch {
	case uninstall:
		return fmt.Sprintf(
			"'dhq setup' is deprecated and will be removed in a future release. "+
				"'dhq skills' has no uninstall yet, so 'dhq setup %s --uninstall' "+
				"remains the way to remove this integration for now.", a.Use)
	case project:
		return fmt.Sprintf(
			"'dhq setup' is deprecated; prefer 'dhq skills install --agent %s'. "+
				"Note 'dhq skills' installs at that agent's default scope rather than "+
				"project-local, and 'dhq setup' will be removed in a future release.", a.SkillsName)
	default:
		return fmt.Sprintf(
			"'dhq setup' is deprecated; use 'dhq skills install --agent %s' instead "+
				"(auto-detects agents and supports 12 of them). 'dhq setup' will be "+
				"removed in a future release.", a.SkillsName)
	}
}

func longHelp(a agentSetup) string {
	userPath, userErr := a.PathFor(scopeUser)
	projPath, _ := a.PathFor(scopeProject)

	var b strings.Builder
	fmt.Fprintf(&b, "Install %s integration files.\n\n", a.Name)
	fmt.Fprintf(&b, "DEPRECATED: use 'dhq skills install --agent %s' instead.\n\n", a.SkillsName)
	if userErr == nil {
		fmt.Fprintf(&b, "Default (user-global): %s\n", userPath)
	} else {
		fmt.Fprintf(&b, "User-global: not supported by %s\n", a.Name)
	}
	fmt.Fprintf(&b, "With --project:        %s\n", projPath)
	return b.String()
}

func runInstall(env *output.Envelope, a agentSetup, path string, sc scope, project bool, strategy writeStrategy) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return &output.InternalError{Message: "create install directory", Cause: err}
	}

	switch strategy {
	case strategyOverwrite:
		if err := os.WriteFile(path, a.Content(), 0644); err != nil {
			return &output.InternalError{Message: "write " + path, Cause: err}
		}
		env.Status("Installed %s integration (%s-level): %s", a.Name, sc, path)
	case strategyMarkedBlock:
		added, err := upsertBlock(path, string(a.Content()))
		if err != nil {
			return &output.InternalError{Message: "update " + path, Cause: err}
		}
		verb := "Updated"
		if added {
			verb = "Added"
		}
		env.Status("%s DeployHQ block in %s (%s-level): %s", verb, a.Name, sc, path)
	}

	uninstallFlag := ""
	if project {
		uninstallFlag = " --project"
	}
	env.Status("\nTo uninstall: dhq setup %s --uninstall%s", a.Use, uninstallFlag)
	return nil
}

func runUninstall(env *output.Envelope, a agentSetup, path string, strategy writeStrategy) error {
	switch strategy {
	case strategyOverwrite:
		if err := os.Remove(path); err != nil {
			if errors.Is(err, os.ErrNotExist) {
				env.Status("%s integration not found at %s", a.Name, path)
				return nil
			}
			return &output.InternalError{Message: "remove " + path, Cause: err}
		}
		// Best-effort cleanup of the deployhq-owned parent dir (e.g. .../skills/deployhq/).
		_ = os.Remove(filepath.Dir(path))
		env.Status("Removed %s integration: %s", a.Name, path)
	case strategyMarkedBlock:
		removed, err := removeBlock(path)
		if err != nil {
			return &output.InternalError{Message: "update " + path, Cause: err}
		}
		if !removed {
			env.Status("No DeployHQ block found in %s", path)
			return nil
		}
		env.Status("Removed DeployHQ block from %s", path)
	}
	return nil
}

// ----- Path resolvers ------------------------------------------------------

func pathClaude(sc scope) (string, error) {
	switch sc {
	case scopeUser:
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(home, ".claude", "skills", "deployhq", "SKILL.md"), nil
	case scopeProject:
		return filepath.Join(".claude", "skills", "deployhq", "SKILL.md"), nil
	}
	return "", errUnsupportedScope
}

func pathCodex(sc scope) (string, error) {
	switch sc {
	case scopeUser:
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(home, ".codex", "AGENTS.md"), nil
	case scopeProject:
		return "AGENTS.md", nil
	}
	return "", errUnsupportedScope
}

func pathCursor(sc scope) (string, error) {
	// Cursor has no user-level rules directory; rules live in .cursor/rules/ per project.
	if sc == scopeUser {
		return "", errUnsupportedScope
	}
	return filepath.Join(".cursor", "rules", "deployhq.mdc"), nil
}

func pathWindsurf(sc scope) (string, error) {
	switch sc {
	case scopeUser:
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(home, ".codeium", "windsurf", "memories", "global_rules.md"), nil
	case scopeProject:
		return filepath.Join(".windsurf", "rules", "deployhq.md"), nil
	}
	return "", errUnsupportedScope
}

// ----- Marked-block helpers ------------------------------------------------

// writeFileAtomic writes data to path via a temp file + rename, so a crash
// mid-write can't corrupt the target. Used for shared files (AGENTS.md,
// global_rules.md) that may contain unrelated user content outside our block.
func writeFileAtomic(path string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".dhq-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer func() { _ = os.Remove(tmpName) }()

	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Chmod(perm); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, path)
}

// upsertBlock inserts or replaces the dhq-managed block in the file at path.
// Returns true if the block was newly added (file created or block appended),
// false if an existing block was replaced.
func upsertBlock(path, content string) (added bool, err error) {
	existing, readErr := os.ReadFile(path)
	if readErr != nil && !errors.Is(readErr, os.ErrNotExist) {
		return false, readErr
	}

	block := blockBegin + "\n" + strings.TrimRight(content, "\n") + "\n" + blockEnd

	if errors.Is(readErr, os.ErrNotExist) {
		return true, writeFileAtomic(path, []byte(block+"\n"), 0644)
	}

	text := string(existing)
	if start := strings.Index(text, blockBegin); start >= 0 {
		rel := strings.Index(text[start:], blockEnd)
		if rel < 0 {
			return false, fmt.Errorf("found %s without matching %s", blockBegin, blockEnd)
		}
		end := start + rel + len(blockEnd)
		return false, writeFileAtomic(path, []byte(text[:start]+block+text[end:]), 0644)
	}

	if !strings.HasSuffix(text, "\n") {
		text += "\n"
	}
	text += "\n" + block + "\n"
	return true, writeFileAtomic(path, []byte(text), 0644)
}

// removeBlock removes the dhq-managed block from the file at path.
// If the file becomes empty afterwards, it is deleted.
func removeBlock(path string) (bool, error) {
	existing, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return false, nil
		}
		return false, err
	}
	text := string(existing)
	start := strings.Index(text, blockBegin)
	if start < 0 {
		return false, nil
	}
	rel := strings.Index(text[start:], blockEnd)
	if rel < 0 {
		return false, fmt.Errorf("found %s without matching %s", blockBegin, blockEnd)
	}
	end := start + rel + len(blockEnd)
	if end < len(text) && text[end] == '\n' {
		end++
	}
	// Also drop a leading blank line that we inserted to separate from prior content.
	if start > 0 && text[start-1] == '\n' && start >= 2 && text[start-2] == '\n' {
		start--
	}
	text = text[:start] + text[end:]
	if strings.TrimSpace(text) == "" {
		return true, os.Remove(path)
	}
	return true, writeFileAtomic(path, []byte(text), 0644)
}

// ----- Skill content -------------------------------------------------------

// skillFrontmatter is the Anthropic-format frontmatter used for the Claude Code
// skill. Other agents either get a different frontmatter (Cursor) or none (Codex,
// Windsurf — they embed the body into shared files like AGENTS.md).
const skillFrontmatter = `---
name: deployhq
description: >
  Deploy code, manage servers, and automate infrastructure via the DeployHQ CLI (dhq).
  Use when the user wants to deploy, check deployment status, manage projects/servers,
  or interact with the DeployHQ platform.
license: MIT
metadata:
  author: DeployHQ
  version: "1.0.0"
  homepage: https://www.deployhq.com/cli
  repository: https://github.com/deployhq/deployhq-cli
---

`

// cursorFrontmatter is the .mdc rule frontmatter Cursor uses to know when to apply
// a rule. alwaysApply=false means Cursor pulls it in only when relevant.
const cursorFrontmatter = `---
description: DeployHQ CLI usage guide — invoke when the user mentions deploys, DeployHQ projects/servers, or the dhq command
alwaysApply: false
---

`

const skillBody = `# DeployHQ CLI — Agent Skill Guide

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
dhq deploy -p <project> -s <server> --wait --json
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
- Use ` + "`--non-interactive`" + ` to guarantee no prompts (auto-enabled for agents and piped output)
- JSON responses include ` + "`breadcrumbs`" + ` with suggested next commands; deploy commands also include resource type and ID
- Error responses include ` + "`retryable`" + `, ` + "`exit_code`" + `, and ` + "`recovery`" + ` actions when applicable
- Exit code 0 = success, 1 = user error, 2 = internal, 3 = auth, 4 = network, 5 = not found, 6 = conflict
- Empty results return exit 0 with empty ` + "`data`" + ` (not an error)
- ` + "`dhq api`" + ` covers any endpoint not in the command tree
- ` + "`dhq commands --json`" + ` includes agent metadata (interactive, destructive, idempotent, safe_for_automation)
- Use ` + "`--wait`" + ` on deploy to block until completion (exits non-zero on failure)

## Triggers
- User mentions "deploy", "deployment", "release" → deployment workflow
- User mentions "server", "hosting" → server management
- User mentions "rollback", "revert" → rollback workflow
- User mentions "env var", "environment", "config" → configuration management
- User mentions "test", "connectivity", "access" → test-access workflow
- User mentions "DeployHQ", "deployhq" → general CLI usage
`
