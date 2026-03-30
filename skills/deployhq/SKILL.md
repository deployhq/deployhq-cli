# DeployHQ CLI — Agent Skill Guide

## Identity
DeployHQ is a deployment automation platform. The `deployhq` CLI manages projects, servers, and deployments.

## Authentication
Set `DEPLOYHQ_API_TOKEN` environment variable, or run `deployhq auth login`.

## Quick Reference

### Discovery
```
deployhq commands --json          # Full command catalog
deployhq --help                   # Usage overview
deployhq <command> --help         # Command details
```

### Common Workflows

#### List projects
```
deployhq projects list --json
```

#### Deploy latest to a server
```
deployhq deploy -p <project> -s <server> --use-latest --json
```

#### Check deployment status
```
deployhq deployments show <id> -p <project> --json
```

#### View deployment logs
```
deployhq deployments logs <id> -p <project>
```

#### Rollback
```
deployhq rollback <deployment-id> -p <project> --json
```

#### Escape hatch (any API endpoint)
```
deployhq api GET /projects/<id>/environment_variables
deployhq api POST /projects/<id>/config_files --body '{"config_file":{...}}'
```

## Decision Tree

### "Deploy code"
1. `deployhq projects list --json` → find project
2. `deployhq servers list -p <project> --json` → find server
3. `deployhq deploy -p <project> -s <server> --json` → create deployment
4. `deployhq deployments show <id> -p <project> --json` → check status

### "Check what's deployed"
1. `deployhq deployments list -p <project> --json` → recent deployments
2. `deployhq deployments show <id> -p <project> --json` → details + steps

### "Something went wrong"
1. `deployhq deployments logs <id> -p <project>` → read logs
2. `deployhq rollback <id> -p <project> --json` → rollback if needed
3. `deployhq deployments abort <id> -p <project>` → abort if running

## Invariants
- Always use `--json` for machine-readable output
- JSON responses include `breadcrumbs` with suggested next commands
- Exit code 0 = success, 1 = failure (check JSON `ok` field for details)
- Empty results return exit 0 with empty `data` (not an error)
- `deployhq api` covers any endpoint not in the command tree

## Triggers
- User mentions "deploy", "deployment", "release" → deployment workflow
- User mentions "server", "hosting" → server management
- User mentions "rollback", "revert" → rollback workflow
- User mentions "DeployHQ", "deployhq" → general CLI usage
