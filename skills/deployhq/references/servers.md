# Servers Reference

## Commands

### `dhq servers list`
List servers in project.

```bash
dhq servers list -p my-app --json
dhq servers list -p my-app --json name,identifier,protocol_type
```

### `dhq servers show <identifier>`
Show server details.

```bash
dhq servers show srv-001 -p my-app --json
```

### `dhq servers create`
Create a server. Flags vary by protocol type.

**Common flags (all protocols):**

| Flag | Required | Description |
|------|----------|-------------|
| `--name` | yes | Server name |
| `--protocol-type` | yes | One of: ssh, ftp, ftps, rsync, s3, s3_compatible, digitalocean, hetzner_cloud, heroku, netlify, shopify, static_hosting (beta), managed_vps (beta) |
| `--path` | no | Deployment path |
| `--environment` | no | Environment label |

**Deployment configuration flags (accepted by both `dhq servers create` and `dhq servers update`):**

| Flag | Values | Description |
|------|--------|-------------|
| `--branch` | branch name, or `""` | The server's **preferred branch** (e.g. `main`, `staging`). `dhq deploy` deploys this branch when `-b/--branch` is not given. Pass `--branch ""` to **unpin** the server so it falls back to the repository default. **Inert on a grouped server** — see the warning below. |
| `--auto-deploy` | `[=true\|false]` (bare flag = `true`) | DeployHQ's **native repository auto-deployment** — DeployHQ deploys this server itself whenever the preferred branch receives a push. |
| `--atomic` | `[=true\|false]` (bare flag = `true`) | Zero-downtime (**atomic**) deployments — each deploy lands in a new release directory that is swapped in at the end. **Read the warning below before setting this.** |
| `--atomic-strategy` | `copy_release` \| `copy_cache` | How the new release directory is built. `copy_release` (default) copies the previous release, then uploads changes into the new release. `copy_cache` uploads changes into a cache directory and copies the new release from there. |
| `--atomic-retention` | integer `>= 1` | How many past releases to keep on the server. Backend default is `3`. The CLI rejects `0` and negative values locally, before any request is sent. |

> **Warning — `--atomic` must be set BEFORE the server's first deployment.**
> The backend refuses to change `atomic` once *any* deployment exists for the
> server, failing with *"cannot be changed after a deployment has been made to
> this server"*. **Do not generate a `dhq servers update ... --atomic` command
> for a server that has already been deployed to** — check
> `dhq deployments list -p <project> --json` first, or set `--atomic` at
> `dhq servers create` time. There is no CLI flag or API call that bypasses
> this; the only remedy is to create a new server with atomic enabled.

Two further backend constraints on `--atomic` — note that they fail *differently*:

| Constraint | What happens when violated |
|------------|----------------------------|
| Supported only on protocol types `ssh`, `rsync`, `digitalocean`, `hetzner_cloud`, `managed_vps` | **Loud** — real validation error: *"not supported on this server type"* |
| The account must have atomic deployments enabled | **Silent** — the request succeeds (2xx) but atomic is *not* applied |

> **Warning — `--atomic` fails SILENTLY on accounts without atomic deployments
> enabled.** When the account is not permitted to use atomic deployments, the
> backend strips `atomic`, `atomic_strategy` and `atomic_retention` from the
> request before validation ever runs. The create/update returns **200/201 with
> no error**, and the server comes back with atomic left off. **A successful
> exit code does not prove atomic was applied.** After enabling it, always read
> the setting back:
>
> ```bash
> dhq servers show srv-001 -p my-app --json atomic,atomic_strategy,atomic_retention
> ```
>
> If `atomic` is `false` after a command that exited 0, the account does not
> have atomic deployments enabled — ask an account admin rather than retrying.
> This is the only atomic failure mode that is silent: an unsupported protocol
> and a change after the first deployment both return real validation errors.

> **Warning — `--branch` is INERT on a server that belongs to a server group.**
> The backend resolves a server's branch as
> `server_group.branch || server.branch || repository.branch`, so the group's
> branch always wins. Grouped servers are also excluded from auto-deployment
> entirely — the *group* is the deploy target, so even when the group's branch
> is blank the deploy uses the **repository** default, never the server's own
> branch. The write still succeeds and the value is echoed back, so this is a
> silent no-op. The CLI now warns on stderr when it detects this
> (`server_group_identifier` is set on the response), but the exit code is
> still 0. **To change the branch for a grouped server, set it on the server
> group, not the member server** — or remove the server from the group first.

**Note on `--auto-deploy`:** native auto-deployment is suppressed while the
server belongs to a server group — the backend only auto-deploys a server when
`auto_deploy` is set *and* the server is not in a group. Setting
`--auto-deploy` on a grouped server stores the preference but has no effect
until the server leaves the group. Same dormancy pattern as `--branch` above.

**Unpinning a branch:** `--branch ""` is sent explicitly (not dropped), and the
backend persists it as an empty string. Every branch consumer treats blank as
"fall back", so the server reverts to the repository default. Note the API
echoes the cleared value back as `""`, not `null` — a read-back check should
treat the two as equivalent.

```bash
# Unpin a server from its branch; it reverts to the repository default
dhq servers update srv-001 -p my-app --branch "" --json
dhq servers show srv-001 -p my-app --json branch,preferred_branch
```

**Static Hosting flags (beta, requires managed-resources beta on account):**

| Flag | Description |
|------|-------------|
| `--subdomain` | Globally unique subdomain under deployhq-sites.com |
| `--spa-mode` | Enable SPA routing (rewrites all paths to index.html) |
| `--subdirectory` | Output subdirectory to publish (e.g. dist) |

**Managed VPS flags (beta, requires managed-resources beta on account):**

| Flag | Description |
|------|-------------|
| `--region` | DigitalOcean region slug (e.g. lon1, nyc3). Use `dhq api GET /managed_hosting/regions` to list. |
| `--size` | DigitalOcean droplet size slug (e.g. s-1vcpu-1gb). Use `dhq api GET /managed_hosting/sizes` to list. |
| `--os-image` | OS image slug (default: ubuntu-24-04-x64) |

**SSH/FTP/FTPS/Rsync flags:**

| Flag | Description |
|------|-------------|
| `--hostname` | Server hostname |
| `--username` | Login username |
| `--password` | Login password |
| `--port` | Connection port |
| `--use-ssh-keys` | Use SSH key auth |
| `--install-key` | Auto-install deploy key |

**S3/S3-Compatible flags:**

| Flag | Description |
|------|-------------|
| `--bucket-name` | S3 bucket name |
| `--access-key-id` | AWS access key |
| `--secret-access-key` | AWS secret key |
| `--custom-endpoint` | Custom S3 endpoint (for S3-compatible) |

**Cloud provider flags:**

| Flag | Provider | Description |
|------|----------|-------------|
| `--personal-access-token` | DigitalOcean | DO API token |
| `--droplet-name` | DigitalOcean | Target droplet |
| `--api-token` | Hetzner Cloud | Hetzner API token |
| `--hetzner-server-name` | Hetzner Cloud | Target server |
| `--app-name` | Heroku | Heroku app name |
| `--api-key` | Heroku | Heroku API key |
| `--site-id` | Netlify | Netlify site ID |
| `--access-token` | Netlify/Shopify | Provider access token |
| `--store-url` | Shopify | Store URL |
| `--theme-name` | Shopify | Theme name |

**Examples:**

```bash
# SSH server
dhq servers create -p my-app --name Production --protocol-type ssh \
  --hostname example.com --username deploy --use-ssh-keys --json

# S3 bucket
dhq servers create -p my-app --name "Static Assets" --protocol-type s3 \
  --bucket-name my-bucket --access-key-id AKIA... --secret-access-key ... --json

# Netlify
dhq servers create -p my-app --name Netlify --protocol-type netlify \
  --site-id abc123 --access-token ... --json

# Static Hosting site (beta)
# Note: requires managed-resources beta enabled; use `dhq launch` for guided setup
dhq servers create -p my-app --name "My Site" --protocol-type static_hosting \
  --subdomain my-app --subdirectory dist --json

# Managed VPS (beta)
# Note: requires managed-resources beta enabled; use `dhq launch` for guided setup
dhq servers create -p my-app --name "My VPS" --protocol-type managed_vps \
  --region lon1 --size s-1vcpu-1gb --accept-cost --json

# Server with deployment configuration set up front
dhq servers create -p my-app --name Production --protocol-type ssh \
  --hostname example.com --username deploy --use-ssh-keys \
  --branch main --auto-deploy --atomic --atomic-retention 5 --json
```

**Worked example — two-environment project (staging + production):**

Staging tracks the `staging` branch and is deployed explicitly from the CLI
(native auto-deployment off). Production tracks `main` and is auto-deployed by
DeployHQ on every push. Both use atomic deployments, so `--atomic` is set here,
at create time — before either server has ever been deployed to.

```bash
# Note: managed_vps is beta and requires managed-resources beta on the account;
# use `dhq launch` for guided setup of a first Managed VPS.

# Staging — preferred branch `staging`, native auto-deploy DISABLED, atomic ON
dhq servers create -p my-app --name Staging --protocol-type managed_vps \
  --region lon1 --size s-1vcpu-1gb --accept-cost \
  --branch staging \
  --auto-deploy=false \
  --atomic --atomic-strategy copy_release --atomic-retention 3 --json

# Production — preferred branch `main`, native auto-deploy ENABLED, atomic ON
dhq servers create -p my-app --name Production --protocol-type managed_vps \
  --region lon1 --size s-2vcpu-2gb --accept-cost \
  --branch main \
  --auto-deploy \
  --atomic --atomic-strategy copy_release --atomic-retention 5 --json

# Verify atomic actually took effect — a 2xx does NOT prove it was applied
# (accounts without atomic deployments enabled have the fields stripped silently).
# Use the identifiers returned by the two create calls above.
dhq servers show <staging-identifier> -p my-app \
  --json atomic,atomic_strategy,atomic_retention
dhq servers show <production-identifier> -p my-app \
  --json atomic,atomic_strategy,atomic_retention

# Staging has no native auto-deployment, so ship it explicitly when you want to.
# No -b needed: the server's preferred branch (`staging`) is used.
dhq deploy -p my-app -s Staging --wait --json

# Production needs no CLI deploy — DeployHQ auto-deploys `main` on push.
# Deploy it manually only when you want an out-of-band release:
dhq deploy -p my-app -s Production --wait --json
```

### `dhq servers update <identifier>`
Update server settings. Accepts the same **deployment configuration flags** as
`dhq servers create` (`--branch`, `--auto-deploy`, `--atomic`,
`--atomic-strategy`, `--atomic-retention`) — see the table in the `create`
section above for values and constraints.

**Only the flags you explicitly pass are changed.** Omitted flags are never sent
to the API, so an update never disturbs a setting the operator did not name.

> **Warning:** `--atomic` cannot be changed on a server that already has a
> deployment — the backend rejects it with *"cannot be changed after a
> deployment has been made to this server"*. Set it at `dhq servers create`
> time. See the full warning in the `create` section above.

```bash
# Rename only — branch, auto-deploy and atomic settings are left untouched
dhq servers update srv-001 -p my-app --name "Production v2" --json

# Change the preferred branch used by `dhq deploy`
dhq servers update srv-001 -p my-app --branch main --json

# Turn DeployHQ's native auto-deployment on / off
dhq servers update srv-001 -p my-app --auto-deploy --json
dhq servers update srv-001 -p my-app --auto-deploy=false --json

# Tune an atomic server's release handling
# (the before-first-deployment rule applies to `--atomic` itself)
dhq servers update srv-001 -p my-app --atomic-strategy copy_cache --json
dhq servers update srv-001 -p my-app --atomic-retention 10 --json

# Enabling atomic — only valid while the server has NO deployments yet,
# and it can succeed with a 2xx without applying, so read the value back
dhq servers update srv-002 -p my-app --atomic --json
dhq servers show srv-002 -p my-app --json atomic,atomic_strategy,atomic_retention
```

### `dhq servers delete <identifier>`
Delete a server.

```bash
dhq servers delete srv-001 -p my-app
```

### `dhq servers reset-host-key <identifier>`
Reset SSH host key verification.

```bash
dhq servers reset-host-key srv-001 -p my-app
```

## Server Groups

### `dhq server-groups list`
```bash
dhq server-groups list -p my-app --json
```

### `dhq server-groups create`
```bash
dhq server-groups create -p my-app --name "US Servers" --json
```

### `dhq server-groups update <identifier>`
```bash
dhq server-groups update grp-001 -p my-app --name "EU Servers" --json
```

### `dhq server-groups delete <identifier>`
```bash
dhq server-groups delete grp-001 -p my-app
```

## Server Name Resolution

When using `--server` / `-s` with `dhq deploy`, names are resolved:
1. Exact case-insensitive match
2. Normalized match (stripped non-alphanumeric)
3. Substring match
4. Interactive picker (TTY) or error (non-TTY)
