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
| `--protocol-type` | yes | One of: ssh, ftp, ftps, rsync, s3, s3_compatible, digitalocean, hetzner_cloud, heroku, netlify, shopify |
| `--path` | no | Deployment path |
| `--environment` | no | Environment label |

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
```

### `dhq servers update <identifier>`
Update server settings.

```bash
dhq servers update srv-001 -p my-app --name "Production v2" --json
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
