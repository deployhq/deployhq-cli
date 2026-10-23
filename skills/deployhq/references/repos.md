# Repository Reference

## Commands

### `dhq repos show`
Show repository configuration for a project.

```bash
dhq repos show -p my-app --json
```

Returns: SCM type, URL, branch, cached status.

### `dhq repos create`
Connect a repository to a project.

| Flag | Required | Description |
|------|----------|-------------|
| `--scm-type` | yes | One of: git, mercurial, subversion |
| `--url` | yes | Repository URL |
| `--branch` | no | Default branch |

```bash
dhq repos create -p my-app --scm-type git --url git@github.com:org/repo.git --json
dhq repos create -p my-app --scm-type git --url git@github.com:org/repo.git --branch main --json
```

**Add the deploy key first for SSH URLs.** For an SSH URL (`git@...` or `ssh://...`)
DeployHQ checks it can read the repository with the project's deploy key the moment
the repository is connected. GitHub refuses SSH access with an unknown key even for
a public repository, so add the key to the repository *before* `repos create`:

```bash
dhq projects show my-app --json=public_key       # the project's PUBLIC deploy key
echo '<public_key>' | gh repo deploy-key add -R org/repo --title DeployHQ -
# or paste it at https://github.com/org/repo/settings/keys/new (read-only is enough)
dhq repos create -p my-app --scm-type git --url git@github.com:org/repo.git --json
```

If the key is missing, `repos create` fails and its error includes the public key,
where to add it, and the command to re-run. HTTPS URLs are not checked when they are
connected, so they do not need a deploy key first.

### `dhq repos update`
Update repository configuration.

```bash
dhq repos update -p my-app --branch main --json
dhq repos update -p my-app --url git@github.com:org/new-repo.git --json
```

### `dhq repos branches`
List repository branches with commit counts.

```bash
dhq repos branches -p my-app --json
```

### `dhq repos commits`
List commits in a branch.

| Flag | Description |
|------|-------------|
| `--branch` | Branch name (default: project default branch) |
| `--limit` | Max commits to return |

```bash
dhq repos commits -p my-app --json
dhq repos commits -p my-app --branch develop --limit 10 --json
```

### `dhq repos commit-info <sha>`
Show details for a single commit.

```bash
dhq repos commit-info abc123def -p my-app --json
```

### `dhq repos latest-revision`
Get the latest revision SHA for the default branch.

```bash
dhq repos latest-revision -p my-app --json
```
