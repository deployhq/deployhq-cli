# Authentication & Setup Reference

## Authentication

**Agents: never ask the person for their API key or password, and never put one
in a command.** Signing in needs the person's API key, a secret that must not
end up in a chat. If `dhq auth status` says `Not logged in`, ask the person to
run `dhq auth login` themselves **in a separate terminal window** (not through
the agent, and not via a `!` shell escape, which has no terminal to prompt in),
then re-check with `dhq auth status`. In CI, set the environment variables below
as secrets instead.

The API key is under **Settings → Security → API Credentials** in DeployHQ
(`https://<account>.deployhq.com/security`).

### `dhq auth login`
Prompts for the account, email and API key (typed hidden) and stores them in the
OS keychain. In non-interactive mode it does not prompt: it stops and says to sign
in from a separate terminal (exit code 3).

| Flag | Description |
|------|-------------|
| `--account` | Account subdomain (pre-fills the prompt) |
| `--email` | Login email (pre-fills the prompt) |
| `--api-key` | API key. Prefer the hidden prompt or `DEPLOYHQ_API_KEY`; never pass a key relayed through an AI chat |

```bash
# Run by the person, in their own terminal
dhq auth login
dhq auth login --account mycompany --email user@example.com   # key still prompted
```

### `dhq auth logout`
Clear stored credentials.

```bash
dhq auth logout
```

### `dhq auth status`
Shows the account dhq would use and checks with DeployHQ that the credentials
work. Credentials are resolved exactly as for API commands: flags, `DEPLOYHQ_*`
environment variables and config files win; the stored login for the same
account fills gaps.

| `status` | Meaning | Next step |
|----------|---------|-----------|
| `authenticated` | Credentials work | Continue |
| `email_verification_required` | Signed in, but DeployHQ refuses every request until the account's email is verified | Ask the person to open the verification link DeployHQ emailed them, then re-run `dhq auth status --json` |
| `invalid_credentials` | DeployHQ rejected the API key (wrong or revoked), or has no such account | Stored login: ask the person to run `dhq auth login` again in a separate terminal. Env/flags/config: fix the values there |
| `api_access_restricted` | Credentials work, but the account's plan has no API access | Upgrade the plan |
| `account_suspended` | Credentials work, but the account is suspended (trial ended, payment outstanding, or suspended by DeployHQ) | Stop and tell the person: `next_step` has the page that resolves it (plans and billing, or support). Re-running will not help |
| `unconfirmed` | Network or server error while checking | Check connectivity and retry |
| `not_logged_in` | No credentials were found: no stored login, and flags/env/config don't set all of account, email and API key. `account` and `source` name the configured account, if any | Ask the person to run `dhq auth login` in a separate terminal, or correct a wrong account setting |

`--json` returns `account`, `email`, `source` (`env`, `flag`, `file`, `stored`
or `mixed`), `sources` (per field), `status`, and `message`/`next_step` when there
is something to fix. Piped output is JSON even without `--json`. Without JSON,
`Not logged in` means no credentials were found (`not_logged_in` in JSON).

```bash
dhq auth status
dhq auth status --json
```

### Unverified email
Until the account's email address is verified, every API request fails with
"Your DeployHQ email address isn't verified yet" (exit code 3, `retryable: true`,
recovery `dhq auth status`). Logging in again does not help: the person has to
open the verification link from their inbox.

### `dhq auth token`
Print the stored API key to stdout (for scripts).

```bash
dhq auth token
```

### Environment Variables (CI)
```bash
export DEPLOYHQ_ACCOUNT=mycompany
export DEPLOYHQ_EMAIL=user@example.com
export DEPLOYHQ_API_KEY=...   # from your CI secret store
```

No login needed when these are set.

### When the account itself refuses API access

Some refusals are not credential problems, so `dhq auth login` will not fix them. Every command (including `auth login` and `hello`) reports them with a message naming the cause and the page that resolves it; in `--json` output, `data.code` identifies the case and `data.recovery[0].url` (also `data.doc_url`) is the page to send a person to:

| `code` | Cause | What resolves it |
|--------|-------|------------------|
| `api_access_restricted` | The account's plan does not include API access (exit code 3) | Upgrade the plan at `https://<account>.deployhq.com/account/packages` |
| `account_suspended` | The trial has ended, or a payment is outstanding (exit code 1) | Choose a plan or settle billing at `https://<account>.deployhq.com/account/packages` |
| `account_suspended` (recovery action `contact_support`) | Suspended for another reason | Contact DeployHQ support |
| `payment_method_required` | The account has no payment method on file for the hosting requested; a Managed VPS always needs one (exit code 1) | Add a card under Payment Details at `https://<account>.deployhq.com/account/payment_details` |

These need a person with access to the account's billing; an agent should stop, report the cause and link, and not retry.

## Signup

### `dhq signup`
Create a new DeployHQ account. **Agents: ask the person to run `dhq signup`
themselves in a separate terminal window**; it asks for the password at a hidden
prompt. Never ask for, or make up, a password.

It asks the person to accept the Terms of Service
(https://www.deployhq.com/terms). In non-interactive mode `--accept-terms` is
required; pass it only when the person has said they accept the terms.

| Flag | Required | Description |
|------|----------|-------------|
| `--email` | non-interactive | Account email |
| `--password` | non-interactive | Account password |
| `--accept-terms` | non-interactive | The person has read and accepted the Terms of Service |
| `--account-name` | no | Requested subdomain (auto-generated if omitted) |
| `--full-name` | no | Full name |

After sign-up:
- **Verify the email.** DeployHQ emails a verification link; until it is opened,
  every API request is refused. `--json` output includes `next_step` when this
  is still needed.
- **Check the account address.** If the requested name was taken or not valid,
  DeployHQ picks another (e.g. `acme` becomes `acme6`). dhq warns about it, and
  `--json` sets `account_name_changed` and `requested_account_name`.
- **Check the credentials were saved.** `--json` sets `credentials_saved`; if it
  is `false`, ask the person to run `dhq auth login` in a separate terminal.

```bash
# Run by the person, in their own terminal
dhq signup
```

## CLI Configuration

### `dhq configure init`
Initialize a config file in the current directory.

```bash
dhq configure init  # creates .deployhq.toml
```

### `dhq config show`
Display current config with sources.

```bash
dhq config show
```

### `dhq configure set <key> <value>`
Set a config value.

```bash
dhq configure set project my-app
dhq configure set account mycompany
```

### `dhq configure unset <key>`
Remove a config value.

```bash
dhq configure unset project
```

### Config File Format (`.deployhq.toml`)
```toml
project = "my-app"
account = "mycompany"
```

### Config Precedence
1. CLI flags (`--project`, `--account`)
2. Environment variables (`DEPLOYHQ_PROJECT`, `DEPLOYHQ_ACCOUNT`)
3. `.deployhq.toml` (current directory, searched up to root)
4. `~/.deployhq/config.toml` (user home)

## Agent Setup

### `dhq skills` (preferred)

Install the DeployHQ skill into the AI coding agents on this machine. `dhq skills`
auto-detects installed agents and supports 12 of them (Aider, Antigravity, Claude
Code, Cline, Codex CLI, Continue.dev, Cursor, Gemini CLI, GitHub Copilot, Kiro CLI,
OpenCode, Windsurf).

```bash
dhq skills list                         # detected agents + skill status
dhq skills install                      # install for detected user-scope agents
dhq skills install --agent claude-code  # install for a specific agent
dhq skills install --agent copilot      # project-scope agents are opt-in via --agent
```

User-scope agents install into your home directory and are covered by the bare
`dhq skills install`; project-scope agents write into the current repository and
require an explicit `--agent` flag.

### `dhq setup` (deprecated)

> **Deprecated:** `dhq setup claude|codex|cursor|windsurf` is the older, narrower
> predecessor of `dhq skills install`. It still works but warns on use and will be
> removed in a future release. Prefer `dhq skills`. Two caveats when migrating:
> `dhq skills` has no uninstall yet (use `dhq setup <agent> --uninstall` to remove),
> and it installs at each agent's own default scope, which may differ from
> `dhq setup --project`.

```bash
dhq setup claude              # ~ dhq skills install --agent claude-code
dhq setup codex               # ~ dhq skills install --agent codex
dhq setup cursor              # ~ dhq skills install --agent cursor
dhq setup windsurf            # ~ dhq skills install --agent windsurf
dhq setup claude --uninstall  # remove (no dhq skills equivalent yet)
```

### `dhq mcp`
Show MCP (Model Context Protocol) server configuration.

```bash
dhq mcp
```

## Telemetry

### `dhq telemetry status`
```bash
dhq telemetry status
```

### `dhq telemetry enable` / `dhq telemetry disable`
```bash
dhq telemetry enable
dhq telemetry disable
```

## Shell Completion

```bash
# Bash
eval "$(dhq completion bash)"

# Zsh
eval "$(dhq completion zsh)"

# Fish
dhq completion fish | source

# PowerShell
dhq completion powershell | Out-String | Invoke-Expression
```

## First-Time Setup

### `dhq hello`
Interactive guided setup for new users (run by the person in their own terminal):
1. Login or signup
2. Project selection
3. Quick orientation

### `dhq init`
Interactive project setup wizard:
1. Project name + SCM + repo URL + branch
2. Deploy key setup
3. Server protocol + credentials
4. Optional first deployment
