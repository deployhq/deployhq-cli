# Changelog

All notable changes to the DeployHQ CLI are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).
While the CLI is pre-1.0, minor versions may carry breaking changes to the public
`pkg/sdk` surface; these are always called out under **Breaking (SDK)**.

## [Unreleased]

### Added

- **`dhq servers create`**: `--key-pair-identifier` provisions a Managed VPS with
  an existing account SSH key, selected by the public identifier that
  `dhq ssh-keys list` prints. Sent as a top-level provisioning param alongside
  `--region`/`--size`/`--os-image`. Omit it to keep the current behaviour, where
  DeployHQ creates and reuses one shared managed key. Rejected locally, before
  any request, for a non-`managed_vps` protocol or alongside
  `--global-key-pair-id` (the ssh/rsync equivalent). Requires the matching API
  change (DHQ-692).
- **SDK**: `ServerCreateRequest.KeyPairIdentifier` (hoisted, `json:"-"`), and
  `ManagedVPSInfo.SSHKey` (`*ManagedVPSSSHKey` — identifier, title, fingerprint)
  for the read-back. Purely additive.

### Fixed

- **Agent skill**: `references/global-resources.md` documented
  `dhq ssh-keys create --name --public-key`; neither flag exists. Keys are
  generated server-side and the flags are `--title` (required) and `--type`
  (`ED25519` default, or `RSA`). Also documents `ssh-keys download -o` and
  `ssh-keys delete`.

- **`dhq servers create` / `dhq servers update`**: five deployment-configuration
  flags — `--branch` (the server's preferred branch), `--auto-deploy`
  (DeployHQ's native repository auto-deployment), `--atomic` (zero-downtime
  deployments), `--atomic-strategy` (`copy_release` — the default — or
  `copy_cache`) and `--atomic-retention` (past releases to keep; must be `>= 1`,
  rejected locally otherwise, backend default `3`). Both commands accept the
  same set; on `update`, only flags that are explicitly passed are sent, so an
  update never disturbs a setting the operator did not name. `--atomic` must be
  configured **before** the server's first deployment — the backend refuses to
  change it once a deployment exists — and is supported only on `ssh`, `rsync`,
  `digitalocean`, `hetzner_cloud` and `managed_vps` servers. On accounts without
  atomic deployments enabled the atomic fields are stripped server-side and the
  request still succeeds, so verify with
  `dhq servers show <id> -p <project> --json atomic,atomic_strategy,atomic_retention`
  rather than trusting the exit code (DHQ-691).
- **`dhq servers create` / `dhq servers update`**: `--branch ""` now unpins a
  server so it falls back to the repository default. Previously the empty value
  was dropped by `omitempty`, and the command reported success having changed
  nothing. The backend accepts and persists a blank branch (it echoes it back as
  `""`, not `null`).
- **`dhq servers create` / `dhq servers update`**: warn on stderr when `--atomic`
  was requested but the server comes back with atomic off. On accounts without
  atomic deployments enabled the backend strips the atomic params before
  validation and returns 2xx, so this was previously a silent success. The
  create/update response is the read-back the docs asked operators to perform,
  so no extra request is made. stdout stays pure data.
- **`dhq servers create` / `dhq servers update`**: warn on stderr when
  `--branch` is set on a server that belongs to a server group. The backend
  resolves the branch as `server_group.branch || server.branch ||
  repository.branch`, and grouped servers are excluded from auto-deployment
  entirely, so a branch stored on a grouped server never deploys — previously a
  silent no-op. stdout stays pure data.
- **SDK**: `ServerCreateRequest` and `ServerUpdateRequest` gained matching
  `Branch`, `AutoDeploy`, `Atomic`, `AtomicStrategy` and `AtomicRetention`
  fields. Purely additive — no existing field changed, so this is not a breaking
  change for importers of `github.com/deployhq/deployhq-cli/pkg/sdk`. `Branch`
  is a `*string` so an explicitly-cleared branch survives serialisation.

## [0.20.1] - 2026-07-24

### Fixed

- **`dhq deploy` / `dhq api`**: prompt with an interactive project picker when
  `-p/--project` is omitted, and detect shell-mangled API paths so `dhq api`
  calls with rewritten slashes still resolve (#35).

### Documentation

- **Agent skill discovery**: removed three stale, frontmatter-less `SKILL.md`
  copies (`.claude/`, `.codex/`, `docs/`) that caused `npx skills add` to emit
  "missing required frontmatter" warnings on every install. The canonical skill
  remains `skills/deployhq/SKILL.md`. Added a skills.sh install badge and section
  to the README, and documented the managed-resources eligibility check
  (`dhq api GET /profile`) and beta-enrollment escape hatch in the launch
  reference (#37).

## [0.20.0] - 2026-07-14

Broad expansion of API coverage (DHQ-639): account-level resources, template
sub-resources, managed hosting, and project/server actions. See #31, #32, #33.

### Breaking (SDK)

These affect external importers of `github.com/deployhq/deployhq-cli/pkg/sdk` and
scripts that parse the JSON output of existing commands. They correct types that
never matched the live API (the old fields were populated from a response shape
the backend does not return), so end-user behaviour of `dhq launch` is fixed
rather than regressed.

- **`ManagedHostingSize`**: renamed `Description` → `Label` and
  `PriceMonthly` → `MonthlyCost`; removed `PriceHourly`, `Memory`, `VCPUs`,
  `Disk`; added `Currency`. Matches the `{ "sizes": [...] }` envelope from
  `GET /managed_hosting/sizes`.
- **`ManagedHostingRegion`**: removed `Available`; added `Flag` and `Country`.
  Regions are now served grouped (`{ "grouped_regions": {...} }`).
- **`Client.UpdateConfigFile`**: now takes `ConfigFileUpdateRequest` (all-pointer
  fields for partial updates) instead of `ConfigFileCreateRequest`.
- **`dhq launch --dry-run --json`**: `monthly_cost` is now currency-aware
  (e.g. `£12.00`, `12.00 SEK`) instead of always `$X.XX`. Update any parser that
  assumes a leading `$`.

### Added

- **Account resources**: `dhq folders`, `dhq users`, `dhq account`,
  `dhq profile`, `dhq api-keys`, `dhq teams` (account-level permission groups),
  and `dhq ssh-keys download` (writes with secure `0600` permissions).
- **Project actions**: `dhq projects regenerate-key`,
  `dhq projects undeployed-changes`, `dhq projects ai-overview`.
- **Server actions**: `dhq servers from-global`, `dhq servers metrics` (beta).
- **Global links**: `dhq config-files link-global|unlink-global` and
  `dhq ssh-commands link-global|unlink-global`.
- **Managed hosting**: `dhq hosted-resources list|show|sync|retry-provision` and
  `dhq managed-hosting regions|sizes`.
- **Template sub-resources**: `dhq templates {config-files, excluded-files,
  integrations, commands, build-commands, build-cache-files, build-known-hosts,
  build-languages, build-configuration, servers, server-groups}`.
- **Utilities**: `dhq ip-ranges`, `dhq plans`, `dhq invoices list|download`,
  `dhq detect`, `dhq beta enroll`.
- **SDK**: `NewPublic(account, …)` client path for public, no-auth endpoints
  (`ip-ranges`, `plans`).

### Changed

- `dhq config-files update` now performs partial updates — unset `--path`/`--body`
  flags are no longer sent, so they are left untouched server-side instead of
  being cleared.

[Unreleased]: https://github.com/deployhq/deployhq-cli/compare/v0.20.1...HEAD
[0.20.1]: https://github.com/deployhq/deployhq-cli/releases/tag/v0.20.1
[0.20.0]: https://github.com/deployhq/deployhq-cli/releases/tag/v0.20.0
