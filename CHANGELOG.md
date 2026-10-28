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

- **SDK**: `APIError` now carries the structured fields some error responses
  include (`ErrorCode`, `Detail`, `UpgradeURL`, `SuspensionType`,
  `SuspensionReason`) and the `AccountURL` the request was sent to, with
  `IsAPIAccessRestricted`, `IsAccountSuspended`, `IsPaymentSuspension` and
  `IsPaymentMethodRequired` helpers. `Message` keeps its existing meaning.

### Changed

- **`dhq templates server-groups create` / `update`**: `--transfer-order` help
  now names the two accepted values, matching the project-level command.
  Behaviour is unchanged.

- **`dhq server-groups` help** no longer claims a deployment reaches every
  server in a group in parallel; it describes the group's transfer order
  instead.

- **`dhq signup`**: `--accept-terms`. Sign-up now asks the person to accept the
  Terms of Service (https://www.deployhq.com/terms) instead of accepting them
  silently; an empty answer is a no. In non-interactive mode `--accept-terms` is
  required, so terms are never accepted without someone saying so. `dhq hello`
  and `dhq launch` sign-up use the same prompt.

- **Root help**: `dhq --help` points AI agents to the setup instructions at
  https://www.deployhq.com/agents.md.

- **Sign-in guidance**: when `dhq auth login` (or any command that needs
  credentials) can't prompt, e.g. under an AI agent, it now tells the person to
  run `dhq auth login` themselves in a separate terminal window, or to set the
  `DEPLOYHQ_*` environment variables for CI, instead of suggesting the API key be
  passed as a flag. It never suggests pasting a key into an AI chat. The three
  "X is required in non-interactive mode" errors of `dhq auth login` are now one
  auth error (exit code 3). The JSON `recovery` action for auth errors is renamed
  from `login` to `ask_user_to_login_in_separate_terminal`.

- **API key location**: help and hints now point to Settings → Security → API
  Credentials (`https://<account>.deployhq.com/security`), where the key
  actually is, instead of "Profile → API Key".

- **`dhq auth status`** checks the credentials with DeployHQ instead of only
  reporting that some are stored, and resolves them exactly as API commands do
  (flags, `DEPLOYHQ_*` environment variables and config files win; the stored
  login for the same account fills gaps), so it reports the account those
  commands use. Previously, with credentials in the environment, it could
  check and report a different, stored account. `status` is now
  `authenticated`, `email_verification_required`, `invalid_credentials` (wrong
  or revoked key, or no such account), `api_access_restricted` (plan without
  API access) or `unconfirmed` (network or server error). `--json` adds
  `source`/`sources` (`env`, `flag`, `file`, `stored`, or `mixed`) and
  `message`/`next_step`; text output has a `Source:` line.

- **Account given as a hostname** (e.g. `DEPLOYHQ_ACCOUNT=acme.deployhq.com`) is
  now normalised to the subdomain when credentials are resolved, so it also
  matches a stored login for that account.

- **Unverified email**: an API request refused because the account's email
  address isn't verified yet now fails with "Your DeployHQ email address isn't
  verified yet..." and a hint to open the verification link (exit code 3,
  `retryable: true`, recovery `dhq auth status`), instead of a raw
  `403 email_verification_required`. `dhq auth login` and `dhq hello` save valid
  credentials and say the email still needs verifying.

- **Sign-up output**: `dhq signup` and `dhq hello` say "check your inbox" when
  the new account's email still needs verifying, and warn when the requested
  account name wasn't available as-is (e.g. `acme` became `acme6`). `dhq signup
  --json` adds `requested_account_name`, `account_name_changed` and `next_step`.
  Non-interactive sign-up errors no longer suggest putting a password in the
  command.

### Fixed

- **Suspended accounts in `dhq auth status` and `dhq launch --dry-run`**: when
  the account is suspended (trial ended, payment outstanding, or suspended by
  DeployHQ), `auth status` now reports `status: "account_suspended"` with the
  cause and the page that resolves it, instead of `unconfirmed` and "re-run".
  `launch --dry-run` now fails the way the real launch does for a suspended
  account or a plan without API access, instead of reporting a successful
  preview.

- **`dhq launch --dry-run`** no longer reports a successful preview for an
  account whose email isn't verified; it fails with the
  `email_verification_required` reason, like the real run.

- **`dhq hello`** sign-up sent no terms acceptance, which the sign-up API
  rejects.

- **Plan and billing refusals are explained**: when the account's plan has no
  API access, its trial has ended, it is suspended, or it has no payment method
  for the hosting requested, every command now says which of these it is and links to the
  page that resolves it (plans, payment details, or support), instead of a
  technical error such as `403 api_access_restricted` or a raw JSON body. In
  `--json` output the error carries a distinct `code` (`api_access_restricted`,
  `account_suspended`, `payment_method_required`), `doc_url`, and a `recovery`
  step with a `url`; a plan restriction no longer suggests `dhq auth login`.
  Exit codes are unchanged. Recovery breadcrumbs gain an optional `url` field,
  and `cmd` is omitted when a step happens in the browser.
- **`dhq launch`**: the plan-eligibility check now runs before a project is
  created or a repository connected, so an ineligible account is stopped before
  anything is left behind. A provisioning refusal for a missing payment method
  now fails with reason `payment_method_required`, a link to the account's
  Payment Details page, and a prompt to re-run the same command; when this run
  created the project, it also names the project, says to re-run with
  `--project` added, and how to delete it. The project is saved to
  `.deployhq.toml` so a re-run reuses it rather than creating a duplicate.

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
