---
name: deployhq
description: |
  Deploy code, manage servers, and automate infrastructure via the DeployHQ CLI (dhq). Use when the user wants to deploy, check deployment status, manage projects/servers, or interact with the DeployHQ platform.
license: MIT
metadata:
  author: DeployHQ
  version: 1.0.0
  homepage: 'https://www.deployhq.com/cli'
  repository: 'https://github.com/deployhq/deployhq-cli'
tags:
  - deployment automation
  - rollback
  - environment variables
  - SSH
  - S3
tools:
  - DeployHQ
  - dhq
---

# DeployHQ CLI — Agent Skill Guide

## Identity

DeployHQ is a deployment automation platform. The `dhq` CLI (binary name: `deployhq`) manages projects, servers, deployments, and infrastructure via the DeployHQ REST API. Designed for both humans and AI agents.

## Authentication

Three methods (checked in order):

1. **Environment variables** (CI/agents): `DEPLOYHQ_ACCOUNT` + `DEPLOYHQ_EMAIL` + `DEPLOYHQ_API_KEY`
2. **Config files**: `~/.deployhq/config.toml` or `.deployhq.toml` in project directory
3. **Interactive login**: `dhq auth login`

Verify with: `dhq auth status`

## Output Contract

**Critical rule**: stdout is ALWAYS data (table or JSON). stderr is ALWAYS human messages.

- **TTY mode**: Table output with headers
- **Piped/non-TTY**: Auto-switches to JSON
- **`--json`**: Force JSON output. Optionally select fields: `--json name,status,identifier`
- **Breadcrumbs**: JSON responses include `breadcrumbs` array with suggested next commands
- **Exit codes**: 0 = success, non-zero = failure

## Non-Interactive Mode

Use `--non-interactive` to guarantee the CLI never prompts. This mode is **auto-enabled** when an agent is detected or output is piped.

In non-interactive mode, any ambiguity (e.g. multiple servers, missing required values) fails with a structured error listing available options instead of prompting.

The only commands that **cannot** run non-interactively are: `dhq init`, `dhq hello`, `dhq configure` (use their flag-based alternatives instead).

## Command Groups

| Group | Description | Reference |
|-------|-------------|-----------|
| **launch** | One command: provision + deploy to DeployHQ Static Hosting or a Managed VPS | [launch.md](references/launch.md) |
| **projects** | Create, list, update, delete projects | [projects.md](references/projects.md) |
| **servers** | Manage deployment targets (SSH, FTP, S3, etc.) | [servers.md](references/servers.md) |
| **deployments** | Create, monitor, rollback deployments | [deployments.md](references/deployments.md) |
| **repos** | Repository configuration, branches, commits | [repos.md](references/repos.md) |
| **configuration** | Env vars, config files, build commands, exclusions, deployment checks, cache files, build languages, known hosts | [configuration.md](references/configuration.md) |
| **global resources** | Global servers, env vars, config files, SSH keys, templates | [global-resources.md](references/global-resources.md) |
| **operations** | Activity, status, test-access, doctor | [operations.md](references/operations.md) |
| **auth & setup** | Authentication, CLI config, agent setup | [auth-setup.md](references/auth-setup.md) |

## Decision Trees

### "Deploy my project in one command (managed hosting)"
The fastest path — provisions DeployHQ Static Hosting or a Managed VPS and deploys in one go:
1. `dhq launch --json` — auto-detects the framework, picks a target, provisions, deploys, returns the live URL
2. Force a target: `dhq launch --static --subdomain my-app --json`, or `dhq launch --vps --accept-cost --region lon1 --json` (a Managed VPS is a managed resource, free for early customers during beta and billed monthly afterwards — `--accept-cost` is required non-interactively)
3. Preview cost/actions with no side effects: `dhq launch --vps --dry-run --json`

See [launch.md](references/launch.md). After `launch` writes `.deployhq.toml`, use `dhq deploy` for subsequent deploys.

### "Deploy code"
1. `dhq projects list --json` — find project permalink
2. `dhq servers list -p <project> --json` — find server identifier
3. `dhq deploy -p <project> -s <server> --json` — create deployment
4. `dhq deployments watch <id> -p <project>` — monitor progress

### "Check what's deployed"
1. `dhq deployments list -p <project> --json` — recent deployments
2. `dhq deployments show <id> -p <project> --json` — details + steps

### "Something went wrong"
1. `dhq deployments logs <id> -p <project>` — read step logs
2. `dhq rollback <id> -p <project> --json` — rollback if needed
3. `dhq deployments abort <id> -p <project>` — abort if running

### "Set up a new project"
1. `dhq projects create --name "My App" --json` — create project
2. `dhq repos create -p <project> --scm-type git --url <repo-url> --json` — connect repo
3. `dhq servers create -p <project> --name Production --protocol-type ssh --hostname <host> --username <user> --json` — add server
4. `dhq deploy -p <project> --json` — first deployment

### "Configure deployment"
1. `dhq env-vars create -p <project> --name KEY --value val` — add env var
2. `dhq config-files create -p <project> --path .env --body "..." --json` — add config file
3. `dhq excluded-files create -p <project> --pattern "node_modules" --json` — add exclusion
4. `dhq build-commands create -p <project> --name "Install" --command "npm install" --json` — add build step
5. `dhq deployment-checks create -p <project> --name "Health" --stage post_deploy --check-type http --http-url https://app.example.com/health --http-expected-status 200 --json` — gate the deploy

### "Escape hatch (any API endpoint)"
```
dhq api GET /projects
dhq api GET /projects/<permalink>/deployments
dhq api POST /projects/<permalink>/deployments --body '{"deployment":{...}}'
```

## Invariants

- Always use `--json` for machine-readable output when scripting or in agent context
- Use `--non-interactive` to guarantee no prompts (auto-enabled for agents and piped output)
- JSON responses include `breadcrumbs` with `action` and `cmd` fields; deploy commands also include `resource` and `id`
- Error responses include `retryable`, `exit_code`, and `recovery` actions when applicable
- Exit codes: 0 = success, 1 = user error, 2 = internal, 3 = auth, 4 = network, 5 = not found, 6 = conflict
- Empty results return exit 0 with empty `data` array (not an error)
- `dhq commands --json` includes per-command agent metadata: `interactive`, `destructive`, `idempotent`, `safe_for_automation`, `resource_types`
- `dhq api` covers all 144+ API endpoints not in the command tree
- Project flag (`-p`/`--project`) accepts permalink or identifier
- Server flag (`-s`/`--server`) uses fuzzy matching: exact > normalized > substring
- Config precedence: flags > env vars > `.deployhq.toml` > `~/.deployhq/config.toml`

## Gotchas

- Some API fields return strings OR numbers inconsistently (handled internally by `FlexString`)
- `dhq deploy` auto-fetches latest revision if `--revision` is omitted
- `dhq deploy` is **incremental by default** — it picks up from the server's last successful deploy. Use `--full` for a full-branch deploy or `--start-revision <sha>` to pin a specific start commit
- `dhq deploy --wait` blocks until deployment completes (use `--timeout` to cap)
- Deployment `watch` uses TUI in TTY mode, append-only in pipes
- `dhq env-vars create` prompts for value if `--value` is omitted (not agent-friendly — always pass `--value`)

## Triggers

- User mentions "deploy", "deployment", "release", "ship" → deployment workflow
- User mentions "server", "hosting", "target" → server management
- User mentions "rollback", "revert", "undo" → rollback workflow
- User mentions "environment variable", "env var", "config", "secret" → configuration
- User mentions "branch", "commit", "repository" → repo management
- User mentions "one command", "just deploy this folder", "managed hosting", "managed VPS", "static hosting", "provision and deploy" → `dhq launch` (see [launch.md](references/launch.md))
- User mentions "DeployHQ", "deployhq", "dhq" → general CLI usage
