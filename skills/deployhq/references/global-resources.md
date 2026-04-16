# Global Resources Reference

Account-level resources shared across projects.

## Global Servers

Server templates that can be copied to any project.

### `dhq global-servers list`
```bash
dhq global-servers list --json
```

### `dhq global-servers show <id>`
```bash
dhq global-servers show 12345 --json
```

### `dhq global-servers create`

| Flag | Required | Description |
|------|----------|-------------|
| `--name` | yes | Server name |
| `--protocol` | yes | One of: ftp, sftp, ssh |
| `--path` | no | Deployment path |
| `--environment` | no | Environment label |

```bash
dhq global-servers create --name "Shared Staging" --protocol ssh --json
```

### `dhq global-servers copy <id>`
Copy a global server into a project.

| Flag | Description |
|------|-------------|
| `-p` | Target project |
| `--name` | Override server name |

```bash
dhq global-servers copy 12345 -p my-app --json
dhq global-servers copy 12345 -p my-app --name "My Staging" --json
```

### `dhq global-servers update <id>`
```bash
dhq global-servers update 12345 --name "Updated Name" --json
```

### `dhq global-servers delete <id>`
```bash
dhq global-servers delete 12345
```

## Global Environment Variables

### `dhq global-env-vars list`
```bash
# Accessed through env-vars with global scope
dhq global-env-vars list --json
```

### `dhq global-env-vars create`
```bash
dhq global-env-vars create --name SHARED_SECRET --value "abc123" --json
```

### `dhq global-env-vars update <id>`
```bash
dhq global-env-vars update 12345 --value "new-value" --json
```

## Global Config Files

Account-level config files shared across projects.

### `dhq global-config-files list`
```bash
dhq global-config-files list --json
```

### `dhq global-config-files show <id>`
```bash
dhq global-config-files show 12345 --json
```

### `dhq global-config-files create`

| Flag | Required | Description |
|------|----------|-------------|
| `--name` | yes | File name |
| `--body` | yes | File content |

```bash
dhq global-config-files create --name ".env.shared" --body "RAILS_ENV=production" --json
```

### `dhq global-config-files update <id>`
```bash
dhq global-config-files update 12345 --body "RAILS_ENV=staging" --json
```

### `dhq global-config-files delete <id>`
```bash
dhq global-config-files delete 12345
```

## SSH Keys

### `dhq ssh-keys list`
```bash
dhq ssh-keys list --json
```

### `dhq ssh-keys create`
```bash
dhq ssh-keys create --name "Deploy Key" --public-key "ssh-rsa AAAA..." --json
```

## Templates

### `dhq templates list`
List custom project templates.

```bash
dhq templates list --json
```

### `dhq templates show <id>`
```bash
dhq templates show 12345 --json
```

### `dhq templates create`
```bash
dhq templates create --name "Rails App" --config '{"..."}' --json
```

### `dhq templates public`
List public (built-in) templates.

```bash
dhq templates public --json
```

### `dhq templates public show <id>`
```bash
dhq templates public show 12345 --json
```

## Other Global Resources

### Zones
```bash
dhq zones list --json
```

### Language Versions
```bash
dhq language-versions list --json
```

### Network Agents
```bash
dhq agents list --json
dhq agents create --name "Office Agent" --json
```

### Integrations
```bash
dhq integrations list -p my-app --json
dhq integrations create -p my-app --type slack --config '{"webhook_url":"..."}' --json
```

### Auto-Deploys
```bash
dhq auto-deploys list -p my-app --json
dhq auto-deploys enable -p my-app --branch main --server Production --json
```

### Scheduled Deploys
```bash
dhq scheduled-deploys list -p my-app --json
dhq scheduled-deploys create -p my-app --cron "0 2 * * *" --branch main --server Production --json
```
