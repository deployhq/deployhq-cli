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
# Login
dhq auth login

# List projects
dhq projects list

# Deploy
dhq deploy -p my-app -s production --use-latest

# Check status
dhq deployments show <id> -p my-app

# View logs
dhq deployments logs <id> -p my-app
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
      - run: dhq deploy --server production --revision ${{ github.sha }} --json true
```

See `examples/github-actions/` for complete workflows:
- **deploy.yml** — Deploy on push to main with polling and failure logs
- **deploy-multi-env.yml** — Staging on push, production on release, auto-rollback
- **deploy-on-pr-merge.yml** — Deploy on PR merge with status comment on the PR

## Commands

```
dhq projects      list | show | create | update | delete | star | insights
dhq servers       list | show | create | update | delete | reset-host-key
dhq server-groups list | show | create | update | delete
dhq deployments   list | show | create | abort | rollback | logs
dhq repos         show | create | update | branches | commits | latest-revision
dhq deploy        (shortcut for deployments create)
dhq rollback      (shortcut for deployments rollback)
dhq open          [project] (open DeployHQ in browser)
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
dhq agents        list | create | update | delete | revoke
dhq ssh-keys      list | create | delete
dhq global-servers list | show | create | update | delete | copy-to-project
dhq global-env-vars list | show | create | update | delete
dhq auto-deploys list | enable
dhq scheduled-deploys list | show | delete
dhq activity      list | stats (account activity — coming soon)
dhq status        (quick dashboard across all projects — coming soon)
dhq assist        (AI deployment assistant, requires Ollama)
dhq completion    bash | zsh | fish | powershell
dhq doctor        (health check)
dhq update        (self-update to latest version)
dhq setup         claude | codex (install agent plugins)
dhq mcp           (start MCP server in stdio mode)
```

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

## Configuration

4 layers (highest to lowest precedence):

1. CLI flags (`--account`, `--project`)
2. Environment variables (`DEPLOYHQ_ACCOUNT`, `DEPLOYHQ_PROJECT`)
3. Project config (`.deployhq.toml` in current directory)
4. Global config (`~/.deployhq/config.toml`)

```bash
# Create project config
dhq config init

# Set default project
dhq config set project my-app

# Show resolved config with sources
dhq config show --resolved
```

## Agent Integration

The CLI is designed for AI agents that can run shell commands.

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
