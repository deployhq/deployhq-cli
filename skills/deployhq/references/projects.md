# Projects Reference

## Commands

### `dhq projects list`
List all projects. Starred projects appear first.

```bash
dhq projects list --json
dhq projects list --json name,permalink,last_deployed_at
```

### `dhq projects show [permalink]`
Show project details including servers. Uses `--project` or positional arg.

```bash
dhq projects show my-app --json
dhq projects show -p my-app --json
```

### `dhq projects create`
Create a new project.

| Flag | Required | Description |
|------|----------|-------------|
| `--name` | yes | Project name |
| `--zone` | no | Deployment zone (see `dhq zones list`) |
| `--template` | no | Template identifier |

```bash
dhq projects create --name "My App" --json
dhq projects create --name "My App" --zone us-east --json
```

### `dhq projects update [permalink]`
Update project settings.

| Flag | Description |
|------|-------------|
| `--name` | New project name |
| `--permalink` | New permalink |
| `--zone` | Deployment zone |
| `--email-notify-on` | Email notification trigger |
| `--notification-email` | Notification email address |
| `--notify-pusher` | Notify code pusher |
| `--check-undeployed-changes` | Check for undeployed changes |
| `--store-artifacts` | Store deployment artifacts |

```bash
dhq projects update my-app --name "My Application" --json
```

### `dhq projects delete [permalink]`
Delete a project. Irreversible.

```bash
dhq projects delete my-app
```

### `dhq projects star [permalink]`
Toggle starred/favourite status.

Aliases: `fav`, `unfav`, `favourite`, `unfavourite`

```bash
dhq projects star my-app --json
```

### `dhq projects insights [permalink]`
Show project deployment insights. JSON only.

```bash
dhq projects insights my-app --json
```

### `dhq projects upload-key [permalink]`
Upload custom SSH public key.

```bash
dhq projects upload-key my-app --key-file ~/.ssh/id_rsa.pub --json
```

### `dhq projects badge [permalink]`
Get deployment status badge URL (SVG).

```bash
dhq projects badge my-app
```
