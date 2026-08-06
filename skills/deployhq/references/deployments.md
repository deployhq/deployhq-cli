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
| `--wait` | `-w` | Block until the deployment completes. **Interactive terminals only** — see the warning below. |
| `--timeout` | | Timeout in seconds for `--wait` (0 = no timeout) |

```bash
# Basic deploy (incremental — only changes since the server's last deploy)
dhq deploy -p my-app --json

# Deploy specific branch to specific server
dhq deploy -p my-app -b staging -s "Staging Server" --json

# Deploy and wait for completion — INTERACTIVE TERMINAL ONLY.
# In a pipe, CI job or agent, --wait does nothing (see the warning below);
# use the create -> watch -> show composition instead.
dhq deploy -p my-app -s Production --wait

# Deploy with timeout (also interactive-only)
dhq deploy -p my-app --wait --timeout 300

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
- `--wait` shows TUI progress in a TTY. It has **no effect** outside one — see the warning below.

> **Warning — `--wait` does not wait unless stdout is a terminal.**
> Output auto-switches to JSON whenever stdout is not a TTY (`WantsJSON()` is
> `JSONMode || !IsTTY`), and the JSON path returns as soon as the deployment is
> **queued**, before the wait begins. So in any pipe, CI job, agent invocation,
> or redirect, `dhq deploy --wait` prints the queued-deployment JSON and exits
> **0 immediately** — the deployment may still be running, and may still fail.
> Passing `--json` explicitly does the same thing; the trigger is the missing
> TTY, not the flag.
>
> **Never treat a zero exit from `dhq deploy` as "the deployment succeeded".**
> Use the composition below, which is safe everywhere.

**Deploy and verify (the safe composition — use this in automation):**

```bash
# 1. create, and capture the identifier
id=$(dhq deploy -p my-app -s Production --json | jq -r '.data.identifier')

# 2. follow it to a terminal state (this genuinely blocks)
dhq deployments watch "$id" -p my-app

# 3. assert the final status — this is the step that actually decides success
status=$(dhq deployments show "$id" -p my-app --json=status | jq -r '.status')
[ "$status" = "completed" ] || { echo "deployment $id ended as: $status"; exit 1; }
```

> **Note — field selection changes the JSON shape.** With `--json` alone the
> response is the full envelope (`{"ok":…, "data":{…}}`), so read values with
> `.data.<field>`. With `--json=<fields>` the envelope is unwrapped and only the
> selected fields are emitted at the **top level** (`{"status":"completed"}`), so
> read them with `.<field>`. Using `.data.status` after `--json=status` yields
> `null`, which would fail the check above on every successful deployment.

Verify the deployed revision and server too when it matters:

```bash
dhq deployments show "$id" -p my-app --json=status,end_revision,server
```

> **Warning — a cancelled deployment exits 0.**
> `dhq deployments watch` returns success for a cancelled deployment
> (`watch.go` treats `cancelled` as a clean terminal state), and `dhq rollback`
> shares the same watcher. Only `failed` produces a non-zero exit. A cancelled
> deployment has **not** deployed your code, so the explicit
> `status == "completed"` assertion in step 3 is required — for rollbacks as
> well as deploys. Do not rely on exit codes alone.

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
