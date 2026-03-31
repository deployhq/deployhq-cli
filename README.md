# deployhq-cli

DeployHQ from your terminal -- for humans and agents.

## Install

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
deployhq update
```

## Quick Start

```bash
# Login
deployhq auth login

# List projects
deployhq projects list

# Deploy
deployhq deploy -p my-app -s production --use-latest

# Check status
deployhq deployments show <id> -p my-app

# View logs
deployhq deployments logs <id> -p my-app
```

## Authentication

```bash
# Interactive login (stores in OS keyring)
deployhq auth login

# Environment variables (CI/agents — no login needed)
export DEPLOYHQ_API_KEY=your-api-key
export DEPLOYHQ_ACCOUNT=your-account
export DEPLOYHQ_EMAIL=your-email
```

## CI/CD (GitHub Actions)

No `deployhq auth login` needed — set secrets and go:

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
      - run: deployhq deploy --server production --revision ${{ github.sha }} --json true
```

See `examples/github-actions/` for complete workflows:
- **deploy.yml** — Deploy on push to main with polling and failure logs
- **deploy-multi-env.yml** — Staging on push, production on release, auto-rollback
- **deploy-on-pr-merge.yml** — Deploy on PR merge with status comment on the PR

## Commands

```
deployhq projects      list | show | create | update | delete | star | insights
deployhq servers       list | show | create | update | delete | reset-host-key
deployhq server-groups list | show | create | update | delete
deployhq deployments   list | show | create | abort | rollback | logs
deployhq repos         show | update | branches | commits | latest-revision
deployhq deploy        (shortcut for deployments create)
deployhq rollback      (shortcut for deployments rollback)
deployhq api           GET|POST|PUT|PATCH|DELETE <path> (escape hatch)
deployhq auth          login | logout | status | token
deployhq config        show | init | set | unset
deployhq commands      (full catalog as JSON for agents)
deployhq show <url>    (show any DeployHQ resource by URL)
deployhq env-vars      list | show | create | update | delete
deployhq config-files  list | show | create | delete
deployhq build-commands list | create | delete
deployhq build-configs list | show | default | delete
deployhq ssh-commands  list | show | create | delete
deployhq excluded-files list | create | delete
deployhq integrations list | show | delete
deployhq agents        list | create | delete | revoke
deployhq global-servers list | show | delete | copy-to-project
deployhq global-env-vars list | show | delete
deployhq auto-deploys list
deployhq scheduled-deploys list | show | delete
deployhq doctor        (health check)
deployhq update        (self-update to latest version)
deployhq setup         claude | codex (install agent plugins)
deployhq mcp           (start MCP server in stdio mode)
```

## JSON Output

All commands support `--json` for machine-readable output:

```bash
# Full JSON
deployhq projects list --json

# Selected fields
deployhq projects list --json name,permalink,zone

# Pipe to jq
deployhq deployments show abc123 -p my-app --json | jq '.data.status'
```

JSON responses include breadcrumbs with suggested next commands:

```json
{
  "ok": true,
  "data": { ... },
  "summary": "Deployment abc123 completed",
  "breadcrumbs": [
    {"action": "logs", "cmd": "deployhq deployments logs abc123 -p my-app"},
    {"action": "rollback", "cmd": "deployhq rollback abc123 -p my-app"}
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
deployhq config init

# Set default project
deployhq config set project my-app

# Show resolved config with sources
deployhq config show --resolved
```

## Agent Integration

The CLI is designed for AI agents that can run shell commands.

```bash
# Install agent plugin
deployhq setup claude

# Full command catalog for agent discovery
deployhq commands --json

# Agent-optimized workflow
DEPLOYHQ_AGENT=my-bot deployhq deploy -p my-app --json
```

Set `DEPLOYHQ_OUTPUT_FILE` to capture all operations as JSONL:

```bash
export DEPLOYHQ_OUTPUT_FILE=/tmp/deployhq.jsonl
deployhq deploy -p my-app
cat /tmp/deployhq.jsonl
```

## Escape Hatch

`deployhq api` covers all 144+ API endpoints:

```bash
deployhq api GET /projects/my-app/environment_variables
deployhq api POST /projects/my-app/config_files --body '{"config_file":{"path":".env","body":"KEY=val"}}'
deployhq api DELETE /projects/my-app/excluded_files/abc123
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
