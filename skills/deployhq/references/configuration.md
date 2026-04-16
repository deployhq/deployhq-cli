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
| `--name` | yes | Command name |
| `--command` | yes | SSH command to execute |
| `--description` | no | Description |

```bash
dhq ssh-commands create -p my-app --name "Restart" --command "sudo systemctl restart app" --json
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
