# Launch Reference (`dhq launch`)

One command that takes a project folder from nothing to a live URL on DeployHQ's
own infrastructure — **Static Hosting** (Cloudflare-backed) or a **Managed VPS**
(DeployHQ-provisioned DigitalOcean droplet). It runs: auth → framework detection
→ beta enrollment (if needed) → repo check → target selection → project/server
creation → provisioning → deploy → prints the live URL → saves `.deployhq.toml`.

Use `dhq launch` for first-time setup of a managed target. Use `dhq deploy` for
subsequent deploys once `.deployhq.toml` exists (launch writes it).

## Usage

```bash
# Interactive cold start (prompts as needed)
dhq launch

# Force a target
dhq launch --static
dhq launch --vps

# Agent / CI (structured JSON result on stdout)
dhq launch --static --subdomain my-app --json
dhq launch --vps --accept-cost --region lon1 --size s-1vcpu-1gb --json

# Inspect intended actions + cost without doing anything (no side effects)
dhq launch --vps --dry-run --json
```

## Flags

| Flag | Description |
|------|-------------|
| `--static` | Force Static Hosting target (skips the target prompt) |
| `--vps` | Force Managed VPS target |
| `--subdomain` | Static Hosting subdomain (default: repo / project name) |
| `--region` | Managed VPS region slug (e.g. `lon1`, `nyc3`). List via `dhq api GET /managed_hosting/regions` |
| `--size` | Managed VPS size slug (e.g. `s-1vcpu-1gb`). List via `dhq api GET /managed_hosting/sizes` |
| `--accept-cost` | Acknowledge Managed VPS provisioning — free for early customers during beta, billed monthly afterwards. **Required** for non-interactive VPS provisioning |
| `--branch` | Branch to deploy (default: repo default) |
| `--project` | Existing project permalink to reuse (skips project creation) |
| `--cleanup-on-failure` | Delete the provisioned server if the deploy fails (prevents orphaned managed resources) |
| `--non-interactive` / `--yes` | Never prompt; fail fast with structured errors. Auto-enabled for agents / piped output |
| `--interactive` | Force prompts even in a piped / agent context |
| `--dry-run` | Print intended actions + monthly cost, do nothing |
| `--json` | Structured result on stdout (global flag) |

## Agent / non-interactive contract

- **Config precedence:** flags → env (`DEPLOYHQ_*`) → `.deployhq.toml` → framework detection. A missing required value with no TTY fails fast naming the exact flag — it never hangs.
- **Cost guardrail:** a Managed VPS is a managed resource (free for early customers during beta, billed monthly afterwards); in non-interactive mode it is **never** provisioned without `--accept-cost` (`--yes` alone is not enough). Static Hosting is safe under `--yes`.
- **`--dry-run`** emits `{would, requires, warning}` and makes no changes — use it to preview cost and confirm before provisioning a Managed VPS.
- **Success (`--json`)** emits one object: `{status, target, url, project, server, deployment}`. In plain mode the final stdout line is the live URL.
- **Idempotent:** re-running reads `.deployhq.toml` and resolves the existing project/server instead of double-provisioning.

### Structured error reasons

On failure the error carries a stable `reason`, a `retryable` boolean, and a `next_step` an agent can branch on:

| Reason | Meaning / next step |
|--------|---------------------|
| `auth_required` | No credentials in non-interactive mode — set `DEPLOYHQ_*` env vars (signup is interactive-only) |
| `beta_enroll_required` | Managed-resources beta not enabled and the user isn't an admin — `details.admin_required=true`; an admin enables it (or use your own server via `dhq init`) |
| `accept_cost_required` | Managed VPS requested non-interactively without `--accept-cost` — re-run with `--accept-cost` |
| `repo_unreachable` | No git remote DeployHQ can deploy from — push a remote / connect a provider first |
| `plan_limit_reached` | Free-plan limit hit (e.g. 1 static site) — upgrade or remove an existing resource |
| `subdomain_taken` | Static Hosting subdomain already in use — choose another `--subdomain` |
| `rate_limited` | Per-account provisioning rate limit hit (HTTP 429) — **retryable** (`retryable: true`); back off for `details.retry_after` seconds and re-run the same command. Distinct from `plan_limit_reached` (a hard 422 cap) |
| `provision_failed` | The server failed to provision — check the named resource; retry |
| `deploy_failed` | Provisioning succeeded but the deploy failed — the managed-resource server is named with its teardown command; `--cleanup-on-failure` removes it automatically |

## Notes

- Static Hosting deploys are git-based (a connected repo + DeployHQ's build pipeline). `launch` connects the repo for you; it does not yet upload a local build directory.
- Managed VPS and Static Hosting require the **managed-resources beta**. `launch` enrolls the account automatically when an admin runs it (the enrollment endpoint is idempotent and admin-gated); non-admins get `beta_enroll_required`.
- **Pricing during beta:** Managed VPS and Static Hosting are free for early customers while in beta; the listed monthly rate applies once the beta ends. The CLI's runtime copy is gated by a single switch (`meteredResourcesInBeta` in `internal/commands/metered.go`) — flip it when the resources go GA, and update this beta wording in the same change.
