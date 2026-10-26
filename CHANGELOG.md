# Changelog

All notable changes to the DeployHQ CLI are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).
While the CLI is pre-1.0, minor versions may carry breaking changes to the public
`pkg/sdk` surface; these are always called out under **Breaking (SDK)**.

## [Unreleased]

### Added

- **`dhq server-groups create` / `dhq server-groups update`**: `--transfer-order`
  (`sequential` or `parallel`) sets how a deployment to the group moves through
  its servers — each server's whole deployment as one unit, or each step
  finished on every server before any server starts the next. Sent as `transfer_order`
  only when given; omitted, nothing is sent, so the backend's default applies on
  create and the group's current order is kept on update. Any other value is
  rejected locally, before any request. A backend that does not yet default
  the field (deployhq/deployhq#1256) refuses a create without it with `422
  transfer_order is not included in the list`; pass the flag there.

- **SDK**: `ServerGroupCreateRequest.TransferOrder` and
  `ServerGroupUpdateRequest.TransferOrder` (`transfer_order`, `omitempty`).
  Purely additive.

- **`dhq projects apply-suggestions [path]`**: adds DeployHQ's suggested build
  settings (build commands, excluded files, build cache files) to any project,
  not only DeployHQ hosting set up by `dhq launch`. Shows what it will add,
  `--dry-run` previews, and re-runs only add what is missing. Build commands are
  added only when the project has none, and stop at the first that fails, with
  the commands to add the rest manually. Exits non-zero if any item could not
  be added.

- **`--server`** on `excluded-files`, `config-files`, `ssh-commands` and
  `integrations` `create` and `update`: target specific servers or server
  groups by name or identifier (repeatable or comma-separated). An unknown or
  ambiguous name fails with the available choices instead of guessing.
- **`--all-servers`** on the same `update` commands. Without either flag, an
  update leaves the existing targeting unchanged.
- **`dhq config-files create --build-only`**: a file that is only available
  to build commands and never uploaded to a server — previously the
  accidental result of every create, now an explicit choice.

### Changed

- **`dhq templates server-groups create` / `update`**: `--transfer-order` help
  now names the two accepted values, matching the project-level command.
  Behaviour is unchanged.

- **`dhq server-groups` help** no longer claims a deployment reaches every
  server in a group in parallel; it describes the group's transfer order
  instead.

- **`dhq projects create`**: checks for a project with the same name first. In
  non-interactive mode it lists the existing project and fails unless
  `--allow-duplicate` is passed; interactively it asks. The new project is saved
  to `.deployhq.toml` in the current directory (never replacing a different
  project already there, a file it can't read, or creating one that would hide a parent folder's `.deployhq.toml`), so later runs in the folder reuse it; `--no-save`
  opts out.
- **`dhq launch`**: saves a newly created project to `.deployhq.toml` right
  away, so a re-run after a later failure reuses it instead of creating
  another, and asks before creating a project whose name already exists.
- **`dhq init`**: if a project with the entered name already exists, the
  wizard lists it and creates another only when the same name is submitted
  again.

- **`dhq excluded-files update`**: `--path` is now optional, so a rule can be
  re-targeted without restating its pattern.
- **`dhq excluded-files list --json`** / **`show`** now include an empty
  `servers` list instead of omitting it, so a rule that applies to no server
  is visible.

### Fixed

- **`dhq config init`**: now writes the `--project`, `--account` and `--email`
  values it is given instead of an empty project entry. `--api-key` is never
  written to a config file.

- **Server targeting on create**: excluded files, config files, SSH commands
  and integrations created with dhq were stored with no target servers, so
  they silently did nothing — an "excluded" file was still uploaded, a config
  file never reached a server, an SSH command never ran, an integration never
  fired. `dhq excluded-files create`, `dhq config-files create`,
  `dhq ssh-commands create`, `dhq integrations create`, the matching
  `dhq templates ... create` commands and the suggested exclusions added by
  `dhq launch` now apply to all servers by default (including servers added
  later). Items created by earlier versions are not changed; re-target them
  with `update <id> --all-servers`.

## [0.21.0] - 2026-08-06

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

### Fixed

- **Agent skill**: corrected commands and semantics that did not match the CLI.
  `dhq deploy --wait` was documented as blocking, but output auto-switches to
  JSON whenever stdout is not a TTY and the JSON path returns as soon as the
  deployment is queued — so in any pipe, CI job or agent it exits 0 immediately
  without waiting. The skill now documents the create → `deployments watch` →
  `deployments show` composition and requires asserting `status == "completed"`.
  Also: a cancelled deployment exits 0 (only `failed` is non-zero), so the
  blanket "non-zero = failure" claim is now qualified; `ssh-commands create`
  cannot set callback phase, server targeting, timeout or halt-on-error, so a
  `post_deploy` SSH deployment check is documented as the first-class
  alternative; and `--json=<fields>` unwraps the response envelope, so selected
  fields are read as `.<field>` rather than `.data.<field>`. Reference-file and
  eval-suite counts in `CLAUDE.md` corrected (DHQ-695).

- **Agent skill**: `references/global-resources.md` documented
  `dhq ssh-keys create --name --public-key`; neither flag exists. Keys are
  generated server-side and the flags are `--title` (required) and `--type`
  (`ED25519` default, or `RSA`). Also documents `ssh-keys download -o` and
  `ssh-keys delete`.

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

[Unreleased]: https://github.com/deployhq/deployhq-cli/compare/v0.21.0...HEAD
[0.21.0]: https://github.com/deployhq/deployhq-cli/releases/tag/v0.21.0
[0.20.1]: https://github.com/deployhq/deployhq-cli/releases/tag/v0.20.1
[0.20.0]: https://github.com/deployhq/deployhq-cli/releases/tag/v0.20.0
