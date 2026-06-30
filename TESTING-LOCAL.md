# Testing `dhq launch` locally (one-command deploy)

How to review/test the one-command deploy work end-to-end against your **local**
DeployHQ. ~10 minutes. Two branches, released together:

| Repo | Branch | PR |
|---|---|---|
| `deployhq` (backend) | `feat/cli-managed-resources-api` | #926 |
| `deployhq-cli` | `feat/one-command-deploy` | #25 |

---

## Prerequisites

- A working **local DeployHQ dev environment** that serves `https://deploy.localhost`
  (the standard `./bin/dev` setup).
- One of: **Go 1.25**, or **Docker** (to build the CLI without installing Go).
- **`gh` CLI authenticated** (`gh auth status`) — exercises the GitHub deploy-key
  auto-install path during launch.
- **A git repo you own** to launch against (the deploy-key step needs a repo you
  control). See [Test repos](#test-repos) to scaffold one.

---

## 1. Check out both branches

Backend:
```bash
cd path/to/deployhq
git fetch origin && git checkout feat/cli-managed-resources-api
bin/rails db:migrate        # pick up any migrations on the branch
```

CLI:
```bash
cd path/to/deployhq-cli
git fetch origin && git checkout feat/one-command-deploy
```

## 2. Start the local backend

```bash
cd path/to/deployhq
./bin/dev
# sanity check (should be a 308 redirect to https://deploy.localhost/):
curl -sI http://deploy.localhost | head -1
```

## 3. Build the CLI dev binary

**With Go 1.25:**
```bash
cd path/to/deployhq-cli
go build -o dhq ./cmd/dhq
```

**Without Go (Docker):** set `GOOS`/`GOARCH` for your machine — Apple Silicon
`darwin/arm64`, Intel mac `darwin/amd64`, Linux `linux/amd64`.
```bash
cd path/to/deployhq-cli
docker run --rm -v "$PWD":/src -v dhqcli-gocache:/go -w /src \
  -e CGO_ENABLED=0 -e GOOS=darwin -e GOARCH=arm64 \
  golang:1.25 go build -o dhq ./cmd/dhq
```

**Verify the build:**
```bash
./dhq version     # MUST print: dhq version dev
```
If it prints `0.x.y`, you're running the released binary, not your build — fix the
alias in the next step.

## 4. Point the CLI at local + alias it

```bash
export DEPLOYHQ_HOST=deploy.localhost
alias dhq="$(pwd)/dhq"      # run from deployhq-cli, or use the absolute path
```

Verify both:
```bash
printenv | grep DEPLOYHQ    # DEPLOYHQ_HOST=deploy.localhost
type dhq                    # dhq is aliased to .../deployhq-cli/dhq
dhq version                 # dhq version dev
```

Why each matters:
- **`DEPLOYHQ_HOST=deploy.localhost`** → the CLI builds its API base URL as
  `https://<account>.deploy.localhost`, i.e. your local backend instead of
  production. Without it, the CLI talks to the hosted product.
- **the alias** → `dhq` runs the dev binary you just built. Any globally-installed
  `dhq` is an old release that doesn't even have a `launch` command.

> Both are **per-shell** — a new terminal needs the `export` + `alias` again (or
> add them to your shell rc).

## 5. Run a launch

From inside a repo you own, with no `.deployhq.toml` yet:

```bash
cd /path/to/some-app
dhq launch
```

**Safe first pass — `--dry-run` (no side effects):**
```bash
dhq launch --dry-run
```
Prints the detected stack, the chosen target (Static Hosting vs Managed VPS), and
— for VPS — the cost, **without creating anything**. Best way to eyeball
detection/steering before touching real infra.

> A real `dhq launch` on a VPS-detected repo **provisions a real DigitalOcean
> droplet** (it costs money and you must delete it afterward). Use `--dry-run`
> first, and `--cleanup-on-failure` on real runs.

---

## What to test

| Scenario | Repo to use | Expected |
|---|---|---|
| **Static steering** | Next.js `output: 'export'`, or a Vite SPA | detected → **Static Hosting** |
| **VPS steering** | Laravel / Rails / generic PHP | detected → **Managed VPS** (`--dry-run` shows VPS + cost) |
| **Signup-in-launch** | logged out / brand-new account | launch walks you through signup, creates the account |
| **Email verification** | account with unverified email | **interactive:** "your email needs to be verified…", waits for you to verify then retries. **non-interactive:** fails cleanly telling you to verify |
| **GitHub deploy key** | a GitHub repo, `gh` authenticated | deploy key auto-installed via `gh` (no manual copy/paste) |
| **Stale config** | repo whose `.deployhq.toml` points at a deleted project/server | CLI detects stale, warns/clears, re-creates |
| **Non-interactive** | `dhq launch --yes` (or piped) | never prompts; VPS requires `--accept-cost`; ambiguity → structured error, never a browser redirect |

### Non-interactive mode

Auto-detected when stdout/stdin aren't a TTY (piped/CI), or forced with
`--non-interactive`:
```bash
dhq launch --non-interactive               # alias: --yes
dhq launch --static --yes                  # force Static, no prompts
dhq launch --vps --accept-cost --yes       # provision a VPS, no prompts (cost ack required)
```
In non-interactive mode the CLI never redirects you to the browser — it fails with
an actionable error instead.

### Useful flags

- `--dry-run` — show intent + cost, no changes
- `--static` / `--vps` — force the target (override detection's choice)
- `--accept-cost` — acknowledge VPS cost (required for non-interactive VPS)
- `--region lon1 --size s-1vcpu-1gb` — VPS placement
- `--subdomain <name>` — Static Hosting subdomain
- `--project <permalink>` — reuse an existing project (skip creation)
- `--branch <name>` — deploy a non-default branch
- `--cleanup-on-failure` — delete the provisioned droplet if the deploy fails

---

## Resetting between runs

- Remove the launch-written config: `rm .deployhq.toml`
- Switch / clear account: edit the `account = …` line in `~/.deployhq/config.toml`,
  or pass `--account <name>`.
- Provisioned a VPS you don't want? **Delete the droplet** — it's real infra.

## Test repos

Detection only needs the manifest, but the deploy-key step needs a repo you own.
Quick scaffolds:

- **Static (Vite SPA):** `npm create vite@latest my-spa -- --template react-ts`
- **VPS (Laravel, no local PHP needed):**
  `docker run --rm -v "$PWD":/app -w /app composer:latest create-project laravel/laravel my-laravel`

Then `git init`, push to a private repo you own
(`gh repo create <you>/my-app --private --source=. --push`), and `dhq launch`
from the checkout.

---

## Known limitation (WIP — not a bug to file)

Managed VPS currently provisions a **bare Ubuntu droplet** and the deploy is a
plain file copy — no web server / PHP runtime is installed yet, so the droplet's
IP will refuse connections after a VPS launch. That runtime-setup step is still in
progress. For reviewing the CLI flow, prefer **`--dry-run`** for the VPS path and
real launches for the **Static Hosting** path.
