# DeployHQ CLI — Agent Skill Guide

## Identity
DeployHQ is a deployment automation platform. The `dhq` CLI manages projects, servers, and deployments.

## Authentication
```
# Guided setup (login or signup, pick a project)
dhq hello

# Environment variables (CI/agents — no login needed)
export DEPLOYHQ_API_KEY=your-api-key
export DEPLOYHQ_ACCOUNT=your-account
export DEPLOYHQ_EMAIL=your-email

# Or interactive login
dhq auth login
```

## Quick Reference

### Discovery
```
dhq commands --json          # Full command catalog
dhq --help                   # Usage overview
dhq <command> --help         # Command details
```

### Common Workflows

#### Deploy and wait for completion
```
dhq deploy -p <project> -s <server> --wait --json
```

#### Deploy latest revision
```
dhq deploy -p <project> -s <server> --use-latest --wait --json
```

#### Check deployment status
```
dhq deployments show <id> -p <project> --json
```

#### Watch a deployment in real-time
```
dhq deployments watch <id> -p <project>
```

#### View deployment logs
```
dhq deployments logs <id> -p <project>
```

#### Rollback
```
dhq rollback <deployment-id> -p <project> --json
```

#### Manage environment variables
```
dhq env-vars list -p <project> --json
dhq env-vars create --name KEY --value VALUE --locked -p <project>
dhq global-env-vars create --name KEY --value VALUE
```

#### Test repository and server connectivity
```
dhq test-access -p <project> --json
dhq test-access -p <project> --server <server> --json
```

#### Escape hatch (any API endpoint)
```
dhq api GET /projects/<id>/environment_variables
dhq api POST /projects/<id>/config_files --body '{"config_file":{...}}'
```

## Decision Tree

### "Deploy code"
1. `dhq projects list --json` → find project
2. `dhq servers list -p <project> --json` → find server
3. `dhq deploy -p <project> -s <server> --wait --json` → deploy and wait
4. On failure: `dhq deployments logs <id> -p <project>` → read logs

### "Check what's deployed"
1. `dhq deployments list -p <project> --json` → recent deployments
2. `dhq deployments show <id> -p <project> --json` → details + steps

### "Something went wrong"
1. `dhq deployments logs <id> -p <project>` → read logs
2. `dhq rollback <id> -p <project> --json` → rollback if needed
3. `dhq deployments abort <id> -p <project>` → abort if running

### "Test connectivity"
1. `dhq test-access -p <project> --json` → test all servers + repo
2. `dhq test-access -p <project> --server <name> --json` → test single server

### "Manage project config"
1. `dhq env-vars list -p <project> --json` → environment variables
2. `dhq config-files list -p <project> --json` → config files
3. `dhq build-commands list -p <project> --json` → build pipeline
4. `dhq ssh-commands list -p <project> --json` → SSH commands
5. `dhq servers list -p <project> --json` → servers

## Invariants
- Always use `--json` for machine-readable output
- JSON responses include `breadcrumbs` with suggested next commands
- Exit code 0 = success, 1 = failure (check JSON `ok` field for details)
- Empty results return exit 0 with empty `data` (not an error)
- `dhq api` covers any endpoint not in the command tree
- Use `--wait` on deploy to block until completion (exits non-zero on failure)

## Triggers
- User mentions "deploy", "deployment", "release" → deployment workflow
- User mentions "server", "hosting" → server management
- User mentions "rollback", "revert" → rollback workflow
- User mentions "env var", "environment", "config" → configuration management
- User mentions "test", "connectivity", "access" → test-access workflow
- User mentions "DeployHQ", "deployhq" → general CLI usage
