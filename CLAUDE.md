# CLAUDE.md

## What is this?

`deployhq-cli` is the official DeployHQ command-line interface — a Go binary that wraps the DeployHQ REST API. Designed for both humans and AI agents.

## Development

```bash
go build ./cmd/deployhq/   # binary outputs as dhq       # Build binary
go test ./... -v               # Run all tests (96 tests)
go vet ./...                   # Static analysis
```

## Architecture

```
pkg/sdk/           Public Go SDK (97 methods, zero internal/ imports)
                    Clean boundary — extractable to standalone module later.

internal/output/    Wrangler-pattern output engine
                    stderr=human messages, stdout=data (table or JSON)
                    --json <fields> for field selection
                    DEPLOYHQ_OUTPUT_FILE for JSONL capture

internal/config/    4-layer Viper config
                    flags > env vars > .deployhq.toml > ~/.deployhq/config.toml

internal/auth/      Keyring + file fallback credential storage

internal/harness/   Agent detection (DEPLOYHQ_AGENT, CLAUDE_CODE, CI)

internal/cli/       Command execution pipeline, lazy SDK client

internal/commands/  All CLI commands (38 top-level, 142 total)

internal/version/   Update checker (GitHub releases API)
```

## Key Design Decisions

- **Output contract**: stdout is ALWAYS data (table or JSON), stderr is ALWAYS human text. Never mixed.
- **`--json <fields>`**: Field selection on any command. Piped output auto-switches to JSON.
- **`FlexString` type**: The DeployHQ API inconsistently returns some fields as strings or numbers. `FlexString` handles both.
- **`dhq api`**: Escape hatch covering all 144+ endpoints not in the command tree.
- **Breadcrumbs**: JSON responses include suggested next commands.
- **No login in CI**: `DEPLOYHQ_ACCOUNT` + `DEPLOYHQ_EMAIL` + `DEPLOYHQ_API_KEY` env vars.

## Testing

- Unit tests use `httptest.NewServer` with recorded response shapes
- `integration_test.go` validates types against real API JSON (golden tests)
- Live staging tests: set env vars and run `/tmp/deployhq-smoke.sh`

## API Type Gotchas

The OpenAPI spec doesn't always match reality. Known divergences:
- `Server.agent` is an object (not string) when populated
- `Server.port` can be string or int depending on the server
- `Timestamps.duration` is an int, not a string
- `DeploymentStep.server` is an int (server ID), not a string
- `DeploymentStep.total_items`/`completed_items` are ints
- `*.servers` arrays on ExcludedFile/ConfigFile/SSHCommand contain objects, not strings
- `DeploymentStepLog.id` is an int, not a string

All handled by `FlexString` or `[]interface{}`.

## Adding a New Resource

1. Add types to `pkg/sdk/types.go` (or new file in `pkg/sdk/`)
2. Add SDK methods (List/Get/Create/Update/Delete)
3. Add CLI command in `internal/commands/`
4. Register in `internal/commands/root.go`
5. Add tests

## Distribution

- GoReleaser: `.goreleaser.yaml` (linux/darwin/windows, amd64/arm64)
- CI: `.github/workflows/ci.yml` (Go 1.23 + 1.24, lint, test -race)
- Release: `.github/workflows/release.yml` (tag push triggers GoReleaser)
- Examples: `examples/github-actions/` (3 deployment workflow patterns)
