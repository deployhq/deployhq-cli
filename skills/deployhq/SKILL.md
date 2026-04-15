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

## Non-Interactive Guarantee

The CLI never prompts when all required flags are provided. For agent usage, always supply all required flags explicitly. The only interactive commands are `dhq init`, `dhq hello`, and `dhq auth login` (when flags are omitted).

## Command Groups

| Group | Description | Reference |
|-------|-------------|-----------|
| **projects** | Create, list, update, delete projects | [projects.md](references/projects.md) |
| **servers** | Manage deployment targets (SSH, FTP, S3, etc.) | [servers.md](references/servers.md) |
| **deployments** | Create, monitor, rollback deployments | [deployments.md](references/deployments.md) |
| **repos** | Repository configuration, branches, commits | [repos.md](references/repos.md) |
| **configuration** | Env vars, config files, build commands, exclusions | [configuration.md](references/configuration.md) |
| **global resources** | Global servers, env vars, SSH keys, templates | [global-resources.md](references/global-resources.md) |
| **operations** | Activity, status, test-access, doctor | [operations.md](references/operations.md) |
| **auth & setup** | Authentication, CLI config, agent setup | [auth-setup.md](references/auth-setup.md) |

## Decision Trees

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

### "Escape hatch (any API endpoint)"
```
dhq api GET /projects
dhq api GET /projects/<permalink>/deployments
dhq api POST /projects/<permalink>/deployments --body '{"deployment":{...}}'
```

## Invariants

- Always use `--json` for machine-readable output when scripting or in agent context
- JSON responses include `breadcrumbs` with suggested next commands — use them for workflow chaining
- Empty results return exit 0 with empty `data` array (not an error)
- `dhq api` covers all 144+ API endpoints not in the command tree
- Project flag (`-p`/`--project`) accepts permalink or identifier
- Server flag (`-s`/`--server`) uses fuzzy matching: exact > normalized > substring
- Config precedence: flags > env vars > `.deployhq.toml` > `~/.deployhq/config.toml`

## Gotchas

- Some API fields return strings OR numbers inconsistently (handled internally by `FlexString`)
- `dhq deploy` auto-fetches latest revision if `--revision` is omitted
- `dhq deploy --wait` blocks until deployment completes (use `--timeout` to cap)
- Deployment `watch` uses TUI in TTY mode, append-only in pipes
- `dhq env-vars create` prompts for value if `--value` is omitted (not agent-friendly — always pass `--value`)

## Triggers

- User mentions "deploy", "deployment", "release", "ship" → deployment workflow
- User mentions "server", "hosting", "target" → server management
- User mentions "rollback", "revert", "undo" → rollback workflow
- User mentions "environment variable", "env var", "config", "secret" → configuration
- User mentions "branch", "commit", "repository" → repo management
- User mentions "DeployHQ", "deployhq", "dhq" → general CLI usage
