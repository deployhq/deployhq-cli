# deployhq-cli

DeployHQ from your terminal -- for humans and agents.

![dhq demo](assets/demo.gif)

## Install

### Homebrew (macOS/Linux)

```bash
brew install deployhq/tap/dhq
```

### Script (macOS/Linux)

```bash
curl -fsSL https://deployhq.com/install/cli | sh
```

### Scoop (Windows)

```powershell
scoop bucket add deployhq https://github.com/deployhq/scoop-bucket
scoop install dhq
```

### Go

```bash
go install github.com/deployhq/deployhq-cli/cmd/dhq@latest
```

### Binary

Download from [Releases](https://github.com/deployhq/deployhq-cli/releases) (Linux, macOS, Windows — amd64/arm64).

### Updating

```bash
dhq update
```

## Quick Start

```bash
# One command: detect the framework, provision DeployHQ hosting
# (Static Hosting or a Managed VPS) and deploy — to a live URL.
dhq launch

# Guided setup (login or signup, pick a project, install the DeployHQ
# skill into your AI coding agents, optional first deploy)
dhq hello

# Or step-by-step
dhq signup
dhq auth login
dhq configure

# Teach your AI coding agents (Claude Code, Cursor, Copilot, …) to drive dhq
dhq skills install

# Deploy and watch in real-time
dhq deploy -p my-app --wait

# Check deployment logs
dhq deployments logs <id> -p my-app

# Open in browser
dhq open my-app
```

## One-command deploy (`dhq launch`)

`dhq launch` takes a project folder to a live URL on DeployHQ's own
infrastructure — **Static Hosting** (global CDN, Cloudflare-backed) or a
**Managed VPS** (DeployHQ-provisioned) — in a single command. It detects your
framework, provisions the target, deploys, and prints the URL.

```bash
dhq launch                    # interactive: detect, pick a target, deploy
dhq launch --static --subdomain my-app
dhq launch --vps --accept-cost --region lon1 --size s-1vcpu-1gb

# Agents / CI — structured JSON, never prompts:
dhq launch --static --json
dhq launch --vps --dry-run --json   # preview cost + actions, no side effects
```

A Managed VPS is a managed resource — free for early customers during the beta,
billed monthly afterwards — so `--accept-cost` is required for non-interactive VPS
provisioning (`--yes` alone never provisions one). After the first run, `launch`
writes `.deployhq.toml` so subsequent deploys are just `dhq deploy`. See the
[agent guide](skills/deployhq/references/launch.md) for the full flag set and the
structured-error reasons agents can branch on.

## Authentication

```bash
# Interactive login (stores in OS keyring)
dhq auth login

# Environment variables (CI/agents — no login needed)
export DEPLOYHQ_API_KEY=your-api-key
export DEPLOYHQ_ACCOUNT=your-account
export DEPLOYHQ_EMAIL=your-email
```

## CI/CD (GitHub Actions)

No `dhq auth login` needed — set secrets and go:

```yaml
# .github/workflows/deploy.yml
env:
  DEPLOYHQ_ACCOUNT: ${{ secrets.DEPLOYHQ_ACCOUNT }}
  DEPLOYHQ_EMAIL: ${{ secrets.DEPLOYHQ_EMAIL }}
  DEPLOYHQ_API_KEY: ${{ secrets.DEPLOYHQ_API_KEY }}
  DEPLOYHQ_PROJECT: my-app

jobs:
  deploy:
    runs-on: ubuntu-latest
    steps:
      - run: curl -fsSL https://deployhq.com/install/cli | sh
      - run: dhq deploy --server production --revision ${{ github.sha }} --wait --json
```

See `examples/github-actions/` for complete workflows:
- **deploy.yml** — Deploy on push to main with polling and failure logs
- **deploy-multi-env.yml** — Staging on push, production on release, auto-rollback
- **deploy-on-pr-merge.yml** — Deploy on PR merge with status comment on the PR

## Commands

```
dhq projects      list | show | create | update | delete | star | insights | upload-key | badge
dhq servers       list | show | create | update | delete | reset-host-key
                  protocols: ssh, ftp, ftps, rsync, s3, s3_compatible, digitalocean,
                             hetzner_cloud, heroku, netlify, shopify,
                             static_hosting (beta), managed_vps (beta)
dhq server-groups list | show | create | update | delete
dhq deployments   list | show | create | abort | rollback | logs | watch
dhq repos         show | create | update | branches | commits | commit-info | latest-revision
dhq deploy        [-p project] [-s server] [--wait] (deploy with live progress)
dhq retry         <deployment-id> -p <project>
dhq rollback      <deployment-id> -p <project>
dhq insights      [project] (deployment insights: totals, success rate, duration)
dhq test-access   [-p project] [-s server] [--wait] (test repo + server connectivity)
dhq open          [project] (open DeployHQ in browser)
dhq hello         (guided onboarding: login/signup + project setup)
dhq init          (interactive project setup wizard)
dhq api           GET|POST|PUT|PATCH|DELETE <path> (escape hatch)
dhq auth          login | logout | status | token
dhq signup        (create a new DeployHQ account)
dhq config        show | init | set | unset
dhq configure     (interactive setup wizard)
dhq commands      (full catalog as JSON for agents)
dhq show <url>    (show any DeployHQ resource by URL)
dhq env-vars      list | show | create | update | delete
dhq config-files  list | show | create | update | delete
dhq build-commands list | create | update | delete
dhq build-configs list | show | default | create | update | delete
dhq ssh-commands  list | show | create | update | delete
dhq deployment-checks list | show | create | update | delete
dhq excluded-files list | show | create | update | delete
dhq integrations list | show | create | update | delete
dhq templates     list | show | public | public-show | create | update | delete
dhq agents        list | create | update | delete | revoke
dhq ssh-keys      list | create | delete
dhq global-servers list | show | create | update | delete | copy-to-project
dhq global-env-vars list | show | create | update | delete
dhq global-config-files list | show | create | update | delete
dhq build-cache-files list | create | update | delete
dhq build-languages set <language-id> --version <ver> [-p project]
dhq build-known-hosts list | create | delete
dhq auto-deploys list | enable
dhq scheduled-deploys list | show | create | update | delete
dhq activity      list | stats
dhq status        (quick dashboard across all projects)
dhq assist        [question] (AI deployment assistant, requires Ollama)
dhq completion    bash | zsh | fish | powershell
dhq doctor        (health check)
dhq update        (self-update to latest version)
dhq skills        list | install (auto-detect AI agents and install the DeployHQ skill)
dhq setup         claude | codex | cursor | windsurf (install agent plugins, --project for project-level)
dhq mcp           (start MCP server in stdio mode)
```

## Deploy with Live Progress

```bash
# Deploy and watch steps in real-time (TUI in interactive terminals)
dhq deploy -p my-app -s production --wait

# Server names are fuzzy-matched
dhq deploy -p my-app -s fedora --wait

# Watch an existing deployment
dhq deployments watch <id> -p my-app
```

On failure, logs are shown automatically with suggested next commands.

## AI Assistant

Get AI-powered help for your deployments using a local LLM. All data stays on your machine.

If you are already using an AI coding agent (Claude Code, Codex, Cursor, etc.), your agent can use `dhq` commands and the API directly — you don't need `dhq assist`. The local assistant is for developers who want a **privacy-first, offline-capable** option using an open-source model via [Ollama](https://ollama.com), without relying on an external coding agent.

```bash
# One-time setup (installs Ollama + downloads model)
dhq assist --setup

# Ask questions about your deployments
dhq assist "why did my deploy fail?" -p my-app
dhq assist "what should I do?" -p my-app
dhq assist "what does transfer_files do?"

# Check status
dhq assist --status
```

Requires [Ollama](https://ollama.com) running locally. Default model: `qwen2.5:3b` (~2GB).

## JSON Output

All commands support `--json` for machine-readable output:

```bash
# Full JSON
dhq projects list --json

# Selected fields
dhq projects list --json name,permalink,zone

# Pipe to jq
dhq deployments show abc123 -p my-app --json | jq '.data.status'
```

JSON responses include breadcrumbs with suggested next commands:

```json
{
  "ok": true,
  "data": { ... },
  "summary": "Deployment abc123 completed",
  "breadcrumbs": [
    {"action": "logs", "cmd": "dhq deployments logs abc123 -p my-app"},
    {"action": "rollback", "cmd": "dhq rollback abc123 -p my-app"}
  ]
}
```

## Shell Completions

```bash
# Zsh (add to ~/.zshrc)
source <(dhq completion zsh)

# Bash (add to ~/.bashrc)
source <(dhq completion bash)

# Fish
dhq completion fish | source
```

Completions include dynamic project and server name suggestions for `--project`, `show`, `open`, and server commands.

## Configuration

4 layers (highest to lowest precedence):

1. CLI flags (`--account`, `--project`)
2. Environment variables (`DEPLOYHQ_ACCOUNT`, `DEPLOYHQ_PROJECT`)
3. Project config (`.deployhq.toml` in current directory)
4. Global config (`~/.deployhq/config.toml`)

```bash
# Interactive setup (recommended)
dhq configure

# Or manual
dhq config init
dhq config set project my-app
dhq config show --resolved
```

## Agent Integration

The CLI is designed for AI agents that can run shell commands.

### Install the DeployHQ skill into your agents

`dhq skills install` detects the AI coding agents on your machine and installs
the DeployHQ skill into each one's native format, so the agent knows how to
drive `dhq`. It's also offered automatically during `dhq hello`.

```bash
# Detect installed agents and show their skill status
dhq skills list

# Install for every detected user-scope agent (Claude Code, Cursor, …)
dhq skills install

# Install for a specific agent (use the name from `dhq skills list`)
dhq skills install --agent claude-code

# Project-scope agents write into the current repo, so they're opt-in:
dhq skills install --agent copilot
```

Twelve agents are supported — Aider, Antigravity, Claude Code, Cline, Codex CLI,
Continue, Cursor, Gemini CLI, GitHub Copilot, Kiro, OpenCode, and Windsurf.
**User-scope** agents install into your home directory and are picked up by the
bare `dhq skills install`; **project-scope** agents write into the current
repository and require an explicit `--agent` flag so login never mutates a repo
as a side effect.

`dhq setup <agent>` is a narrower alternative that installs the agent plugin for
a single tool (Claude Code, Codex, Cursor, or Windsurf), with `--project` for
project-level installs.

### Other agent helpers

```bash
# Full command catalog with agent safety metadata
dhq commands --json

# Agent-optimized workflow
DEPLOYHQ_AGENT=my-bot dhq deploy -p my-app --json
```

Standalone skill guides are also available at `.claude/SKILL.md`,
`.codex/SKILL.md`, `.cursor/SKILL.md`, `.windsurf/SKILL.md`, and `docs/SKILL.md`.

### Non-Interactive Mode

Use `--non-interactive` to guarantee the CLI never prompts. Auto-enabled when output is piped.

```bash
# Explicit strict mode — errors instead of prompting
dhq deploy -p my-app --non-interactive --json

# Piped output auto-enables non-interactive
dhq deploy -p my-app --json | jq .
```

### Agent Metadata

`dhq commands --json` includes per-command safety metadata:

```json
{
  "agent": {
    "interactive": false,
    "destructive": true,
    "idempotent": false,
    "requires_confirmation": true,
    "supports_json": true,
    "safe_for_automation": true,
    "resource_types": ["project"]
  }
}
```

Set `DEPLOYHQ_OUTPUT_FILE` to capture all operations as JSONL:

```bash
export DEPLOYHQ_OUTPUT_FILE=/tmp/deployhq.jsonl
dhq deploy -p my-app
cat /tmp/deployhq.jsonl
```

### Skill System

The `skills/deployhq/` directory contains structured reference docs that AI agents consume to correctly use the CLI:

- `SKILL.md` — Entry point: auth, output contract, decision trees, gotchas
- `references/` — 8 per-domain docs (projects, servers, deployments, repos, configuration, global resources, operations, auth/setup)

### Skill Evals

`skill-evals/deployhq/` contains 57 evaluation cases that test whether an LLM correctly translates natural language into `dhq` commands:

```bash
# Dry-run (no API calls)
./skill-evals/deployhq/run-evals.sh --dry-run

# Run all evals
ANTHROPIC_API_KEY=sk-... ./skill-evals/deployhq/run-evals.sh

# Run one category
./skill-evals/deployhq/run-evals.sh --category deployments

# Test a specific model
./skill-evals/deployhq/run-evals.sh --model claude-haiku-4-5-20251001
```

## Escape Hatch

`dhq api` covers all 144+ API endpoints:

```bash
dhq api GET /projects/my-app/environment_variables
dhq api POST /projects/my-app/config_files --body '{"config_file":{"path":".env","body":"KEY=val"}}'
dhq api DELETE /projects/my-app/excluded_files/abc123
```

## Go SDK

The SDK at `pkg/sdk/` is a clean public interface:

```go
import "github.com/deployhq/deployhq-cli/pkg/sdk"

client, _ := sdk.New("myco", "user@example.com", "api-key")
projects, _ := client.ListProjects(ctx)
dep, _ := client.CreateDeployment(ctx, "my-app", sdk.DeploymentCreateRequest{
    Branch: "main",
})
```

## Development

```bash
go build ./cmd/dhq/
go test ./... -v
go vet ./...
```

## License

MIT
