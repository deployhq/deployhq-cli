---
name: deployhq
description: |
  Deploy code, manage servers, and automate infrastructure via the DeployHQ CLI (dhq). Use when the user wants to deploy, check deployment status, manage projects/servers, or interact with the DeployHQ platform.
license: MIT
metadata:
  author: DeployHQ
  version: 1.0.0
  homepage: 'https://www.deployhq.com/cli'
  repository: 'https://github.com/deployhq/deployhq-cli'
tags:
  - deployment automation
  - rollback
  - environment variables
  - SSH
  - S3
tools:
  - DeployHQ
  - dhq
---

# DeployHQ CLI — Agent Skill Guide

## Identity

DeployHQ is a deployment automation platform. The `dhq` CLI (binary name: `deployhq`) manages projects, servers, deployments, and infrastructure via the DeployHQ REST API. Designed for both humans and AI agents.

## Working With the Person

DeployHQ deploys the person's code to places they care about, so the person makes the decisions and the agent does the legwork. In practice that looks like this:

- **The person chooses where the code goes.** When there is more than one sensible option, the agent describes the options, asks, and waits for the answer. A missing answer is not a choice.
- **One step at a time.** Setup is a series of small commands (create a project, connect a repository, add a server, deploy). The agent says what the next step does and checks with the person before steps that are hard to undo: creating resources, giving DeployHQ access to a private repository, changing anything on the person's server, starting paid or beta resources, and deploying.
- **Secrets stay out of the chat.** The agent does not ask for API keys or passwords, and does not put them on a command line it runs. Anything that needs a secret (`dhq auth login`, `dhq signup`, a password-only server) is done by the person in their own terminal or in the DeployHQ web app.
- **The person's details come from the person.** The agent asks for the DeployHQ login email and account name rather than guessing them (for example from git settings).
- **The first deploy is the person's call.** The agent summarises what will be deployed and where, asks before running it, and reports success only once the deployment's status is `completed`.

## Authentication

Check first: `dhq auth status --json`. If it reports a signed-in account, setup can continue.

If not, the first question is **"Do you already have a DeployHQ account?"**

- **Yes:** the person signs in by running `dhq auth login` in a separate terminal window of their own (not through the agent). It prompts for the account name, the login email and an API key, and hides the key as it is typed. When they say they are done, the agent runs `dhq auth status --json` to confirm.
- **No:** with the person's OK, they create an account either on the DeployHQ website or by running `dhq signup` in their own terminal (it prompts for the email and password, hiding the password, asks them to accept the Terms of Service, and saves the new credentials). DeployHQ then sends a verification email; the person verifies before anything else will work. Signing up on the website does not sign in the CLI, so after verifying, that person also runs `dhq auth login` in their own terminal. The agent runs `dhq auth status --json` afterwards.

`dhq auth login` and `dhq signup` cannot prompt inside an agent (non-interactive mode), so run there they would need the key or password on the command line. That is why the person runs them.

Other ways credentials are picked up (checked in this order):

1. **Environment variables** (CI): `DEPLOYHQ_ACCOUNT` + `DEPLOYHQ_EMAIL` + `DEPLOYHQ_API_KEY`
2. **Config files**: `~/.deployhq/config.toml` or `.deployhq.toml` in project directory
3. **Stored login** from `dhq auth login` / `dhq signup`

`dhq auth status --json` checks the credentials with DeployHQ. Its `status`
must be `authenticated` before you continue; `email_verification_required` means the
person has to open the verification link DeployHQ emailed them (every API
request is refused until then). The API key is under Settings → Security → API
Credentials in DeployHQ. See [auth-setup.md](references/auth-setup.md).

## Output Contract

**Critical rule**: stdout is ALWAYS data (table or JSON). stderr is ALWAYS human messages.

- **TTY mode**: Table output with headers
- **Piped/non-TTY**: Auto-switches to JSON
- **`--json`**: Force JSON output. Optionally select fields: `--json=name,status,identifier`
- **Breadcrumbs**: JSON responses include `breadcrumbs` array with suggested next commands
- **Exit codes**: 0 = success, non-zero = failure — with one exception: a **cancelled** deployment also exits 0. Never infer deployment success from the exit code alone; assert `status == "completed"` via `dhq deployments show <id> --json=status`

## Non-Interactive Mode

Use `--non-interactive` to guarantee the CLI never prompts. This mode is **auto-enabled** when an agent is detected or output is piped.

In non-interactive mode, any ambiguity (e.g. multiple servers, missing required values) fails with a structured error listing available options instead of prompting.

`dhq hello`, `dhq init` and `dhq configure` are interactive wizards: inside an agent they refuse to run. The agent uses the individual commands instead (`dhq projects create`, `dhq repos create`, `dhq servers create`, `dhq config set`). `dhq auth login` and `dhq signup` only work non-interactively when the secret is passed as a flag, so the person runs them in their own terminal (see Authentication).

## Command Groups

| Group | Description | Reference |
|-------|-------------|-----------|
| **launch** | All-in-one setup for DeployHQ Static Hosting or a Managed VPS (an option when the person picks DeployHQ hosting) | [launch.md](references/launch.md) |
| **projects** | Create, list, update, delete projects | [projects.md](references/projects.md) |
| **servers** | Manage deployment targets (SSH, FTP, S3, etc.) | [servers.md](references/servers.md) |
| **deployments** | Create, monitor, rollback deployments | [deployments.md](references/deployments.md) |
| **repos** | Repository configuration, branches, commits | [repos.md](references/repos.md) |
| **configuration** | Env vars, config files, build commands, exclusions, deployment checks, cache files, build languages, known hosts | [configuration.md](references/configuration.md) |
| **global resources** | Global servers, env vars, config files, SSH keys, templates | [global-resources.md](references/global-resources.md) |
| **operations** | Activity, status, test-access, doctor | [operations.md](references/operations.md) |
| **auth & setup** | Authentication, CLI config, agent setup | [auth-setup.md](references/auth-setup.md) |

## Decision Trees

### "Set up DeployHQ for this project" (first time)
Small steps, each one something the person can approve on its own:
1. **Account.** `dhq auth status --json`. If not signed in, follow Authentication above (ask whether they have an account; they sign in or sign up in their own terminal).
2. **Existing project?** `dhq projects list --json`, and check for a `.deployhq.toml` in the folder. If a matching project exists, ask whether to reuse it rather than creating a second one.
3. **Where to deploy.** Ask, and wait for the answer. The usual options:
   - **Their own server** (SSH/SFTP, FTP, S3 and others): most DeployHQ customers deploy this way.
   - **DeployHQ hosting**: Static Hosting for static sites, or a Managed VPS. These are beta features that the account has to be enrolled in, can need a payment method on the account, and are billed once the beta ends.
4. **Create the project** (after the person agrees): `dhq projects create --name "<name>" --json`, then `dhq config set project <permalink>` so later sessions find it.
5. **Connect the repository** (after the person agrees; for a private repository this gives DeployHQ read access): `dhq repos create -p <project> --scm-type git --url <repo-url> --json`. See [repos.md](references/repos.md) for the access key a private repository needs.
6. **Add the target.**
   - Own server: DeployHQ connects with the project's access key (`dhq projects show <project> --json=public_key`). Installing that key on the server changes who can log in to it, so the agent shows the key and asks before adding it, or the person adds it themselves. Then `dhq servers create ...` (see [servers.md](references/servers.md)). If the server only accepts a password, the person adds it in the DeployHQ web app so the password never passes through the chat.
   - DeployHQ hosting: `dhq beta enroll --protocol static_hosting` (needs an account admin) and `dhq servers create -p <project> --protocol-type static_hosting --subdomain <name> ...`, or a `managed_vps` server. Check with the person before each.
7. **Configure** build commands, excluded files, env vars as the project needs (see "Configure deployment").
8. **First deploy.** Summarise the project, branch and target, ask, then `dhq deploy -p <project> -s <server> --json`.
9. **Confirm.** `dhq deployments watch <id> -p <project>` follows the deployment until it finishes (a new one may start as `queued`), then `dhq deployments show <id> -p <project> --json=status`; report success only for `completed`.

`dhq launch` does steps 4 to 9 for DeployHQ hosting in a single command. It is an option when the person has chosen DeployHQ hosting and wants it done in one go, not the default; see [launch.md](references/launch.md).

### "Deploy code"
1. `dhq projects list --json` — find project permalink
2. `dhq servers list -p <project> --json` — find server identifier
3. `dhq deploy -p <project> -s <server> --wait --json` — create the deployment (check with the person first) and wait for its final status
4. Report success only if the JSON shows `"status": "completed"` (a non-zero exit means it failed or timed out)

### "Check what's deployed"
1. `dhq deployments list -p <project> --json` — recent deployments
2. `dhq deployments show <id> -p <project> --json` — details + steps

### "Something went wrong"
1. `dhq deployments logs <id> -p <project>` — read step logs
2. `dhq rollback <id> -p <project> --json` — rollback if needed
3. `dhq deployments abort <id> -p <project>` — abort if running

### "Set up a new project"
Same steps as "Set up DeployHQ for this project" above, checking with the person before each one:
1. `dhq projects create --name "My App" --json` — create project (refuses if one with that name exists; saves it to `.deployhq.toml` here, only inside a git repository)
2. `dhq repos create -p <project> --scm-type git --url <repo-url> --json` — connect repo
   (SSH URL such as `git@github.com:...`: add the project's `public_key` to the repo as a deploy key FIRST, or the connect is rejected — see references/repos.md)
3. `dhq projects apply-suggestions -p <project> --json` — add DeployHQ's suggested build commands and exclusions
4. `dhq servers create -p <project> --name Production --protocol-type ssh --hostname <host> --username <user> --json` — add server
5. `dhq deploy -p <project> --json` — first deployment, after the person says go

### "Configure deployment"
1. `dhq env-vars create -p <project> --name KEY --value val` — add env var
2. `dhq config-files create -p <project> --path .env --body "..." --json` — add config file
3. `dhq excluded-files create -p <project> --path "node_modules"` — add exclusion (all servers unless `--server`)
4. `dhq build-commands create -p <project> --name "Install" --command "npm install" --json` — add build step
5. `dhq deployment-checks create -p <project> --name "Health" --stage post_deploy --check-type http --http-url https://app.example.com/health --http-expected-status 200 --json` — gate the deploy

### "Escape hatch (any API endpoint)"
```
dhq api GET /projects
dhq api GET /projects/<permalink>/deployments
dhq api POST /projects/<permalink>/deployments --body '{"deployment":{...}}'
```

## Invariants

- Always use `--json` for machine-readable output when scripting or in agent context
- Use `--non-interactive` to guarantee no prompts (auto-enabled for agents and piped output)
- JSON responses include `breadcrumbs` with `action` and `cmd` fields (or `url` instead of `cmd` for a step done in the browser, such as adding a payment method); deploy commands also include `resource` and `id`
- Error responses include `retryable`, `exit_code`, and `recovery` actions when applicable
- Exit codes: 0 = success, 1 = user error, 2 = internal, 3 = auth, 4 = network, 5 = not found, 6 = conflict
- Empty results return exit 0 with empty `data` array (not an error)
- `dhq commands --json` includes per-command agent metadata: `interactive`, `destructive`, `idempotent`, `requires_confirmation` (ask the person before running), `safe_for_automation` (deterministic in non-interactive mode and fine to run without a person watching; false for commands that print secrets), `resource_types`
- `dhq api` covers all 144+ API endpoints not in the command tree
- Project flag (`-p`/`--project`) accepts permalink or identifier
- Server flag (`-s`/`--server`) uses fuzzy matching: exact > normalized > substring
- Config precedence: flags > env vars > `.deployhq.toml` > `~/.deployhq/config.toml`

## Gotchas

- Some API fields return strings OR numbers inconsistently (handled internally by `FlexString`)
- `dhq deploy` auto-fetches latest revision if `--revision` is omitted
- `dhq deploy` is **incremental by default** — it picks up from the server's last successful deploy. Use `--full` for a full-branch deploy or `--start-revision <sha>` to pin a specific start commit
- Without `--wait`, `dhq deploy` returns as soon as the deployment is **queued** — that is not success. Use `--wait` (blocks in every output mode; exits non-zero on failure or `--timeout`) and confirm the final status is `completed` before reporting the deploy as done (see [deployments.md](references/deployments.md))
- Deployment `watch` uses TUI in TTY mode, append-only in pipes
- `dhq env-vars create` prompts for value if `--value` is omitted (not agent-friendly — always pass `--value`)
- `dhq servers create` / `dhq servers update` configure deployment behaviour with `--branch` (preferred branch), `--auto-deploy` (DeployHQ's native auto-deployment), `--atomic`, `--atomic-strategy` and `--atomic-retention`. On `update`, only flags you explicitly pass are sent — omitted flags never disturb existing settings
- **`--atomic` must be set before the server's first deployment.** The backend refuses to change it once any deployment exists ("cannot be changed after a deployment has been made to this server") and there is no override — the only remedy is a new server. Set it at `create` time; check `dhq deployments list -p <project> --json` before ever putting `--atomic` on an `update`
- `--atomic` fails **silently** on accounts without atomic deployments enabled: the fields are stripped server-side and the request still returns 2xx with atomic off. A zero exit code does not prove it applied — read it back with `dhq servers show <id> -p <project> --json=atomic,atomic_strategy,atomic_retention`

## Triggers

- User mentions "deploy", "deployment", "release", "ship" → deployment workflow
- User mentions "server", "hosting", "target" → server management
- User mentions "rollback", "revert", "undo" → rollback workflow
- User mentions "environment variable", "env var", "config", "secret" → configuration
- User mentions "branch", "commit", "repository" → repo management
- User mentions "set up DeployHQ", "deploy this site/repo" for the first time → "Set up DeployHQ for this project": ask about their account, then where to deploy
- User mentions "managed hosting", "managed VPS", "static hosting" → DeployHQ hosting; offer the step-by-step path or `dhq launch` and let the person choose (see [launch.md](references/launch.md))
- User mentions "DeployHQ", "deployhq", "dhq" → general CLI usage
