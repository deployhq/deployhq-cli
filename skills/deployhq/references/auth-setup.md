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

### `dhq skills` (preferred)

Install the DeployHQ skill into the AI coding agents on this machine. `dhq skills`
auto-detects installed agents and supports 12 of them (Aider, Antigravity, Claude
Code, Cline, Codex CLI, Continue.dev, Cursor, Gemini CLI, GitHub Copilot, Kiro CLI,
OpenCode, Windsurf).

```bash
dhq skills list                         # detected agents + skill status
dhq skills install                      # install for detected user-scope agents
dhq skills install --agent claude-code  # install for a specific agent
dhq skills install --agent copilot      # project-scope agents are opt-in via --agent
```

User-scope agents install into your home directory and are covered by the bare
`dhq skills install`; project-scope agents write into the current repository and
require an explicit `--agent` flag.

### `dhq setup` (deprecated)

> **Deprecated:** `dhq setup claude|codex|cursor|windsurf` is the older, narrower
> predecessor of `dhq skills install`. It still works but warns on use and will be
> removed in a future release. Prefer `dhq skills`. Two caveats when migrating:
> `dhq skills` has no uninstall yet (use `dhq setup <agent> --uninstall` to remove),
> and it installs at each agent's own default scope, which may differ from
> `dhq setup --project`.

```bash
dhq setup claude              # ~ dhq skills install --agent claude-code
dhq setup codex               # ~ dhq skills install --agent codex
dhq setup cursor              # ~ dhq skills install --agent cursor
dhq setup windsurf            # ~ dhq skills install --agent windsurf
dhq setup claude --uninstall  # remove (no dhq skills equivalent yet)
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
