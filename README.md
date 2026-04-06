# deployhq-cli

DeployHQ from your terminal -- for humans and agents.

## Install

### Homebrew (macOS/Linux)

```bash
brew install deployhq/tap/dhq
```

### Script (macOS/Linux)

```bash
curl -fsSL https://raw.githubusercontent.com/deployhq/deployhq-cli/main/install.sh | sh
```

### Go

```bash
go install github.com/deployhq/deployhq-cli/cmd/deployhq@latest
```

### Binary

Download from [Releases](https://github.com/deployhq/deployhq-cli/releases).

### Updating

```bash
dhq update
```

## Quick Start

```bash
# Guided setup (login or signup, pick a project, optional first deploy)
dhq hello

# Or step-by-step
dhq signup
dhq auth login
dhq configure

# Deploy and watch in real-time
dhq deploy -p my-app --wait

# Check deployment logs
dhq deployments logs <id> -p my-app

# Open in browser
dhq open my-app
```

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
      - run: curl -fsSL https://raw.githubusercontent.com/deployhq/deployhq-cli/main/install.sh | sh
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
dhq server-groups list | show | create | update | delete
dhq deployments   list | show | create | abort | rollback | logs | watch
dhq repos         show | create | update | branches | commits | commit-info | latest-revision
dhq deploy        [-p project] [-s server] [--wait] (deploy with live progress)
dhq retry         <deployment-id> -p <project>
dhq rollback      <deployment-id> -p <project>
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
dhq excluded-files list | show | create | update | delete
dhq integrations list | show | create | update | delete
dhq templates     list | show | public | public-show | create | update | delete
dhq agents        list | create | update | delete | revoke
dhq ssh-keys      list | create | delete
dhq global-servers list | show | create | update | delete | copy-to-project
dhq global-env-vars list | show | create | update | delete
dhq auto-deploys list | enable
dhq scheduled-deploys list | show | create | update | delete
dhq activity      list | stats
dhq status        (quick dashboard across all projects)
dhq assist        [question] (AI deployment assistant, requires Ollama)
dhq completion    bash | zsh | fish | powershell
dhq doctor        (health check)
dhq update        (self-update to latest version)
dhq setup         claude | codex (install agent plugins)
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

Agent skill guides are available at `.claude/SKILL.md`, `.codex/SKILL.md`, and `docs/SKILL.md`.

```bash
# Install agent plugin
dhq setup claude

# Full command catalog for agent discovery
dhq commands --json

# Agent-optimized workflow
DEPLOYHQ_AGENT=my-bot dhq deploy -p my-app --json
```

Set `DEPLOYHQ_OUTPUT_FILE` to capture all operations as JSONL:

```bash
export DEPLOYHQ_OUTPUT_FILE=/tmp/deployhq.jsonl
dhq deploy -p my-app
cat /tmp/deployhq.jsonl
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
go build ./cmd/deployhq/
go test ./... -v
go vet ./...
```

## License

MIT
