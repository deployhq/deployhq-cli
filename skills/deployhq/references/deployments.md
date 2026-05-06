---
tags:
  - deployment
  - rollback
  - retry
  - abort
  - build cache
tools:
  - dhq
---
# Deployments Reference

## Commands

### `dhq deploy`
Shortcut for creating a deployment with smart defaults.

| Flag | Short | Description |
|------|-------|-------------|
| `--branch` | `-b` | Branch to deploy |
| `--server` | `-s` | Server or group identifier (fuzzy matched) |
| `--revision` | `-r` | End revision SHA (auto-fetches latest if omitted) |
| `--start-revision` | | Start revision SHA (default: server's last deployed commit) |
| `--full` | | Deploy entire branch from the first commit (overrides incremental default) |
| `--wait` | `-w` | Block until deployment completes |
| `--timeout` | | Timeout in seconds for `--wait` (0 = no timeout) |

```bash
# Basic deploy (incremental — only changes since the server's last deploy)
dhq deploy -p my-app --json

# Deploy specific branch to specific server
dhq deploy -p my-app -b staging -s "Staging Server" --json

# Deploy and wait for completion
dhq deploy -p my-app -s Production --wait --json

# Deploy with timeout
dhq deploy -p my-app --wait --timeout 300 --json

# Deploy a specific commit range (e.g. a hotfix)
dhq deploy -p my-app --start-revision a1b2c3d --revision e4f5g6h --json

# Force a full deploy (entire branch from the first commit)
dhq deploy -p my-app --full --json
```

**Behaviors:**
- Auto-selects sole server; prompts for multiple (TTY) or errors (non-TTY)
- Branch resolution: `--branch` overrides everything. Otherwise deploys the **server's default branch** (`preferred_branch`). Falls back to the repo default only if the server has no preferred branch.
- Revision resolution: `--revision` overrides everything. Otherwise uses the tip SHA of the resolved branch.
- **Start revision resolution (incremental by default):** `--full` forces an empty start (full-branch deploy). Otherwise `--start-revision` is used if set. Otherwise the resolved server's `last_revision` (last successful deploy) is used — this is what makes deploys incremental. Servers with no prior deploy and server-group identifiers fall through to a full deploy because there's no single baseline to start from.
- An unknown `--branch` errors out instead of silently deploying the wrong branch.
- `--full` and `--start-revision` are mutually exclusive.
- `--wait` shows TUI progress in TTY, append-only in pipes

### `dhq deployments list`
List recent deployments with pagination.

```bash
dhq deployments list -p my-app --json
dhq deployments list -p my-app --json identifier,status,branch,created_at
```

### `dhq deployments show <identifier>`
Show deployment details including steps.

```bash
dhq deployments show abc123 -p my-app --json
```

### `dhq deployments create`
Full deployment creation with all options.

| Flag | Description |
|------|-------------|
| `--branch` | Branch to deploy |
| `--revision` | End revision SHA |
| `--server` | Target server |
| `--parent` | Parent revision SHA |
| `--copy-config` | Copy config files from previous deployment |
| `--run-build` | Execute build commands |
| `--use-cache` | Use build cache |

```bash
dhq deployments create -p my-app --branch main --server Production --json
```

### `dhq deployments abort <identifier>`
Abort a running deployment.

```bash
dhq deployments abort abc123 -p my-app --json
```

### `dhq deployments retry <identifier>`
Retry a failed or completed deployment.

```bash
dhq deployments retry abc123 -p my-app --json
```

### `dhq deployments rollback <identifier>`
Rollback a deployment.

```bash
dhq deployments rollback abc123 -p my-app --json
```

Shortcut: `dhq rollback <identifier> -p <project> --json`

### `dhq deployments logs <deployment-id>`
Show deployment step logs.

| Flag | Description |
|------|-------------|
| `--step` | Specific step number (shows all if omitted) |

```bash
dhq deployments logs abc123 -p my-app
dhq deployments logs abc123 -p my-app --step 2
```

### `dhq deployments watch <deployment-id>`
Watch deployment progress in real-time.

```bash
dhq deployments watch abc123 -p my-app
```

**Behaviors:**
- TTY: Full TUI with step-by-step progress and emoji status
- Non-TTY: Append-only output for CI/log capture
- Auto-shows logs for failed steps

## Shortcuts

| Command | Equivalent |
|---------|------------|
| `dhq deploy ...` | `dhq deployments create ...` (with smart defaults) |
| `dhq retry <id>` | `dhq deployments retry <id>` |
| `dhq rollback <id>` | `dhq deployments rollback <id>` |

## Deployment Statuses

| Status | Meaning |
|--------|---------|
| `pending` | Queued, waiting to start |
| `running` | In progress |
| `completed` | Finished successfully |
| `failed` | Finished with errors |
| `cancelled` | Aborted by user |
