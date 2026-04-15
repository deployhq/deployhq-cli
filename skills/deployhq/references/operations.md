# Operations Reference

Monitoring, diagnostics, and utility commands.

## Activity

### `dhq activity list`
View recent deployment activity/events.

```bash
dhq activity list --json
dhq activity list -p my-app --json
```

### `dhq activity stats`
Show deployment statistics.

```bash
dhq activity stats --json
dhq activity stats -p my-app --json
```

## Status

### `dhq status`
Dashboard view of project health: recent deployments, server status, project summary.

```bash
dhq status -p my-app --json
```

## Test Access

### `dhq test-access`
Test server connectivity.

```bash
# Test all servers in project
dhq test-access -p my-app --json

# Test specific server
dhq test-access -p my-app --server Production --json
```

### `dhq test-access show <result-id>`
Show detailed test result.

```bash
dhq test-access show abc123 -p my-app --json
```

## Doctor

### `dhq doctor`
Diagnose CLI configuration and environment. Shows:
- CLI version
- Config files loaded
- Credentials status
- Git status (if in repo)
- Network connectivity

```bash
dhq doctor
```

## Raw API Access

### `dhq api <method> <path>`
Escape hatch for any API endpoint.

| Argument | Description |
|----------|-------------|
| `<method>` | HTTP method: GET, POST, PUT, PATCH, DELETE |
| `<path>` | API path |

| Flag | Description |
|------|-------------|
| `--body` | JSON request body |

```bash
dhq api GET /projects
dhq api GET /projects/my-app/deployments
dhq api POST /projects/my-app/deployments --body '{"deployment":{"branch":"main"}}'
dhq api DELETE /projects/my-app/servers/12345
```

## URL Utilities

### `dhq url parse <url>`
Parse a DeployHQ URL into components.

```bash
dhq url parse "https://myaccount.deployhq.com/projects/my-app/deployments/abc123"
```

Returns: account, resource, project, sub_resource, id.

### `dhq show <url>`
Fetch and display any DeployHQ resource by URL. Auto-detects resource type.

```bash
dhq show "https://myaccount.deployhq.com/projects/my-app" --json
```

## Utility

### `dhq open [permalink]`
Open project in browser.

```bash
dhq open my-app
dhq open  # opens current project (from .deployhq.toml)
```

### `dhq commands-catalog`
List all CLI commands and subcommands. Useful for agent introspection.

```bash
dhq commands-catalog --json
```

### `dhq version`
```bash
dhq version
```

### `dhq update`
Update CLI to latest version.

```bash
dhq update
```
