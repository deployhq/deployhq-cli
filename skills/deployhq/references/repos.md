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

**Note:** After creation, the response includes a deploy key that must be added to your Git provider.

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
