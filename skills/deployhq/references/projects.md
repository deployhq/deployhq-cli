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
| `--allow-duplicate` | no | Create it even if a project with the same name exists |
| `--no-save` | no | Don't save the new project to `.deployhq.toml` |

```bash
dhq projects create --name "My App" --json
dhq projects create --name "My App" --zone us-east --json
```

**Duplicate names:** DeployHQ accepts several projects with the same name, so
`create` checks first. If one exists, it lists it and fails in non-interactive
mode (asks in interactive mode). Reuse the existing project
(`dhq config set project <permalink>`) rather than passing `--allow-duplicate`,
unless the user really wants a second one.

**Directory link:** the new project is saved as `project` in `.deployhq.toml` in
the current directory (an existing link to a different project is left alone,
with a warning). Later commands in that folder, including a fresh agent session,
then use it without `--project`. Before creating a project, check
`dhq config show` — if the folder already names a project, use it.

### `dhq projects apply-suggestions [path]`
Detect the framework of a local directory (default `.`) and add DeployHQ's
suggested build settings to the project: build commands, excluded files (e.g.
dependency folders) and build cache files. Works for any project, whatever
servers it deploys to. See
[configuration.md](configuration.md#suggested-build-settings).

```bash
dhq projects apply-suggestions -p my-app --dry-run --json
dhq projects apply-suggestions -p my-app --json
```

### `dhq config init`
Create `.deployhq.toml` in the current directory (or `--global` for
`~/.deployhq/config.toml`). `--project`, `--account` and `--email` are written to
the file. The API key never is — use `dhq auth login` or `DEPLOYHQ_API_KEY`.
Fails if the file already exists; then use `dhq config set <key> <value>`.

```bash
dhq config init --project my-app --account my-company
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
