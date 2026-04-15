# Authentication & Setup Reference

## Authentication

### `dhq auth login`
Login interactively or with flags.

| Flag | Description |
|------|-------------|
| `--account` | Account subdomain |
| `--email` | Login email |
| `--api-key` | API key |

```bash
# Interactive (prompts for missing fields)
dhq auth login

# Non-interactive (agent-friendly)
dhq auth login --account mycompany --email user@example.com --api-key abc123
```

### `dhq auth logout`
Clear stored credentials.

```bash
dhq auth logout
```

### `dhq auth status`
Show current authentication info.

```bash
dhq auth status
dhq auth status --json
```

### `dhq auth token`
Display current API token (masked).

```bash
dhq auth token
```

### Environment Variables (CI/Agent)
```bash
export DEPLOYHQ_ACCOUNT=mycompany
export DEPLOYHQ_EMAIL=user@example.com
export DEPLOYHQ_API_KEY=abc123
```

No login needed when these are set.

## Signup

### `dhq signup`
Create new DeployHQ account.

| Flag | Required | Description |
|------|----------|-------------|
| `--email` | yes | Account email |
| `--password` | yes | Account password |
| `--account-name` | no | Custom subdomain (auto-generated if omitted) |

```bash
dhq signup --email user@example.com --password "..."
```

## CLI Configuration

### `dhq configure init`
Initialize a config file in the current directory.

```bash
dhq configure init  # creates .deployhq.toml
```

### `dhq configure show`
Display current config with sources.

```bash
dhq configure show
```

### `dhq configure set <key> <value>`
Set a config value.

```bash
dhq configure set project my-app
dhq configure set account mycompany
```

### `dhq configure unset <key>`
Remove a config value.

```bash
dhq configure unset project
```

### Config File Format (`.deployhq.toml`)
```toml
project = "my-app"
account = "mycompany"
```

### Config Precedence
1. CLI flags (`--project`, `--account`)
2. Environment variables (`DEPLOYHQ_PROJECT`, `DEPLOYHQ_ACCOUNT`)
3. `.deployhq.toml` (current directory, searched up to root)
4. `~/.deployhq/config.toml` (user home)

## Agent Setup

### `dhq setup claude`
Install Claude Code integration files.

| Flag | Description |
|------|-------------|
| `--project` | Install to project directory (`.claude/`) instead of user (`~/.claude/`) |
| `--uninstall` | Remove integration files |

```bash
dhq setup claude
dhq setup claude --project
dhq setup claude --uninstall
```

### `dhq setup codex`
Install OpenAI Codex integration.

```bash
dhq setup codex
```

### `dhq setup cursor`
Install Cursor integration.

```bash
dhq setup cursor
```

### `dhq setup windsurf`
Install Windsurf integration.

```bash
dhq setup windsurf
```

### `dhq mcp`
Show MCP (Model Context Protocol) server configuration.

```bash
dhq mcp
```

## Telemetry

### `dhq telemetry status`
```bash
dhq telemetry status
```

### `dhq telemetry enable` / `dhq telemetry disable`
```bash
dhq telemetry enable
dhq telemetry disable
```

## Shell Completion

```bash
# Bash
eval "$(dhq completion bash)"

# Zsh
eval "$(dhq completion zsh)"

# Fish
dhq completion fish | source

# PowerShell
dhq completion powershell | Out-String | Invoke-Expression
```

## First-Time Setup

### `dhq hello`
Interactive guided setup for new users:
1. Login or signup
2. Project selection
3. Quick orientation

### `dhq init`
Interactive project setup wizard:
1. Project name + SCM + repo URL + branch
2. Deploy key setup
3. Server protocol + credentials
4. Optional first deployment
