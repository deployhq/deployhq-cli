---
tags:
  - environment variables
  - config files
  - build commands
  - build configuration
  - deployment checks
tools:
  - dhq
  - snyk
  - trivy
---
# Configuration Reference

Project-level configuration: environment variables, config files, build commands, and exclusions.

## Environment Variables

### `dhq env-vars list`
List environment variables (values are masked).

```bash
dhq env-vars list -p my-app --json
```

### `dhq env-vars create`
Create an environment variable.

| Flag | Required | Description |
|------|----------|-------------|
| `--name` | yes | Variable name |
| `--value` | no | Variable value (prompts if omitted — not agent-friendly) |
| `--locked` | no | Lock variable (hides value after creation) |

```bash
dhq env-vars create -p my-app --name DATABASE_URL --value "postgres://..." --json
dhq env-vars create -p my-app --name SECRET_KEY --value "abc123" --locked --json
```

**Agent note:** Always pass `--value` explicitly. Omitting it triggers an interactive prompt.

### `dhq env-vars update <id>`
```bash
dhq env-vars update 12345 -p my-app --value "new-value" --json
```

### `dhq env-vars delete <id>`
```bash
dhq env-vars delete 12345 -p my-app
```

## Config Files

### `dhq config-files list`
```bash
dhq config-files list -p my-app --json
```

### `dhq config-files show <id>`
Show file path and content.

```bash
dhq config-files show 12345 -p my-app --json
```

### `dhq config-files create`

| Flag | Required | Description |
|------|----------|-------------|
| `--path` | yes | File path on server |
| `--body` | yes | File content |
| `--description` | no | Description |

```bash
dhq config-files create -p my-app --path ".env" --body "APP_ENV=production" --json
```

### `dhq config-files update <id>`
```bash
dhq config-files update 12345 -p my-app --body "APP_ENV=staging" --json
```

### `dhq config-files delete <id>`
```bash
dhq config-files delete 12345 -p my-app
```

## Build Commands

### `dhq build-commands list`
```bash
dhq build-commands list -p my-app --json
```

### `dhq build-commands create`

| Flag | Required | Description |
|------|----------|-------------|
| `--name` | yes | Command name |
| `--command` | yes | Shell command to execute |
| `--description` | no | Description |

```bash
dhq build-commands create -p my-app --name "Install" --command "npm install" --json
dhq build-commands create -p my-app --name "Build" --command "npm run build" --json
```

### `dhq build-commands update <id>`
```bash
dhq build-commands update 12345 -p my-app --name "Install deps" --command "npm ci" --json
```

## Build Configs

### `dhq build-configs list`
```bash
dhq build-configs list -p my-app --json
```

### `dhq build-configs create`

| Flag | Required | Description |
|------|----------|-------------|
| `--name` | yes | Config name |
| `--commands` | yes | Comma-separated command IDs |
| `--description` | no | Description |

```bash
dhq build-configs create -p my-app --name "Node Build" --commands "123,456" --json
```

## Excluded Files

### `dhq excluded-files list`
```bash
dhq excluded-files list -p my-app --json
```

### `dhq excluded-files create`

| Flag | Required | Description |
|------|----------|-------------|
| `--pattern` | yes | File/directory pattern to exclude |
| `--description` | no | Description |

```bash
dhq excluded-files create -p my-app --pattern "node_modules" --json
dhq excluded-files create -p my-app --pattern ".git" --json
dhq excluded-files create -p my-app --pattern "*.log" --json
```

## SSH Commands

### `dhq ssh-commands list`
```bash
dhq ssh-commands list -p my-app --json
```

### `dhq ssh-commands create`

| Flag | Required | Description |
|------|----------|-------------|
| `--command` | yes | SSH command to execute |
| `--description` | no | Description shown in the UI |
| `--timing` | no | When the command runs: `all` (default), `first`, or `after_first` |

```bash
dhq ssh-commands create -p my-app --command "sudo systemctl restart app" --description "Restart" --json
```

### `dhq ssh-commands update <id>`

Update fields on an existing SSH command. Only flags you pass are sent — omit `--timing` to leave the current value untouched.

| Flag | Required | Description |
|------|----------|-------------|
| `--command` | no | New SSH command to execute |
| `--description` | no | New description |
| `--timing` | no | New timing: `all`, `first`, or `after_first` |

```bash
dhq ssh-commands update cmd_abc123 -p my-app --command "sudo systemctl reload app" --json
```

## Deployment Checks

Gate a deployment at a stage. A check has a `stage` (`pre_build` or `post_deploy`) and a `check_type`:
- `ssh` — runs a command over SSH on selected servers
- `http` — sends an HTTP request from the deployment worker
- `vulnerability_scan` — runs a security scanner on the build server (pre_build only)

### `dhq deployment-checks list`
```bash
dhq deployment-checks list -p my-app --json
```

### `dhq deployment-checks show <id>`
```bash
dhq deployment-checks show chk_abc123 -p my-app --json
```

### `dhq deployment-checks create`

| Flag | Required | Description |
|------|----------|-------------|
| `--name` | yes | Display name |
| `--stage` | yes | `pre_build` or `post_deploy` |
| `--check-type` | yes | `ssh`, `http`, or `vulnerability_scan` |
| `--enabled` | no | Whether the check runs (default `true`) |
| `--timeout` | no | Timeout in seconds |
| `--description` | no | Description |
| `--command` | ssh | Command to run on the target servers |
| `--servers` | ssh | Server identifiers to target (repeat or comma-separate) |
| `--http-method` | http | HTTP method (e.g. `GET`) |
| `--http-url` | http | URL to probe |
| `--http-expected-status` | http | Expected status code |
| `--http-body-match` | http | Substring expected in the response body |
| `--scanner` | vuln | `snyk`, `trivy`, or `custom` |
| `--scan-target-kind` | vuln | Target kind |
| `--scan-target` | vuln | Target path or identifier |
| `--severity-threshold` | vuln | Minimum severity that fails the check |
| `--fail-on-unfixed-only` | vuln | Only fail on findings with no available fix |
| `--sarif-output-path` | vuln | Where the scanner writes SARIF output |

SSH check:
```bash
dhq deployment-checks create -p my-app \
  --name "Run migrations" --stage post_deploy --check-type ssh \
  --command "bundle exec rails db:migrate" --servers srv_prod1,srv_prod2 --json
```

HTTP check:
```bash
dhq deployment-checks create -p my-app \
  --name "Health check" --stage post_deploy --check-type http \
  --http-method GET --http-url https://app.example.com/health --http-expected-status 200 --json
```

Vulnerability scan (pre_build only):
```bash
dhq deployment-checks create -p my-app \
  --name "Snyk scan" --stage pre_build --check-type vulnerability_scan \
  --scanner snyk --severity-threshold high --fail-on-unfixed-only --json
```

### `dhq deployment-checks update <id>`

Partial update — only flags you pass are sent.

```bash
# Disable temporarily
dhq deployment-checks update chk_abc123 -p my-app --enabled=false --json

# Tighten the severity gate
dhq deployment-checks update chk_abc123 -p my-app --severity-threshold critical --json
```

### `dhq deployment-checks delete <id>`
```bash
dhq deployment-checks delete chk_abc123 -p my-app
```

## Build Cache Files

Manage files/directories cached between builds to speed up deployments.

### `dhq build-cache-files list`
```bash
dhq build-cache-files list -p my-app --json
```

### `dhq build-cache-files create`

| Flag | Required | Description |
|------|----------|-------------|
| `--path` | yes | Path to cache |

```bash
dhq build-cache-files create -p my-app --path "node_modules" --json
dhq build-cache-files create -p my-app --path ".cache" --json
```

### `dhq build-cache-files update <id>`
```bash
dhq build-cache-files update 12345 -p my-app --path "vendor" --json
```

### `dhq build-cache-files delete <id>`
```bash
dhq build-cache-files delete 12345 -p my-app
```

## Build Languages

Set language runtime versions used by the build server.

### `dhq build-languages set <language-id>`

| Flag | Required | Description |
|------|----------|-------------|
| `--version` | yes | Language version |
| `--build-config` | no | Build configuration override ID |

```bash
# Set Ruby version for project
dhq build-languages set ruby -p my-app --version "3.2" --json

# Set Node version for a specific build config override
dhq build-languages set node -p my-app --version "20" --build-config override-123 --json
```

**Tip:** Use `dhq language-versions list -p <project>` to see available versions.

## Build Known Hosts

Manage SSH known hosts for the build server (e.g. for private dependencies fetched during builds).

### `dhq build-known-hosts list`
```bash
dhq build-known-hosts list -p my-app --json
```

### `dhq build-known-hosts create`

| Flag | Required | Description |
|------|----------|-------------|
| `--hostname` | yes | SSH hostname |
| `--public-key` | yes | SSH public key |

```bash
dhq build-known-hosts create -p my-app --hostname "github.com" --public-key "ssh-rsa AAAA..." --json
```

### `dhq build-known-hosts delete <id>`
```bash
dhq build-known-hosts delete 12345 -p my-app
```
