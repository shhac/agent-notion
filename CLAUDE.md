# agent-notion

Notion CLI for humans and LLMs, written in Go on the shared `lib-agent-*`
family (same stack as the sibling repos `agent-mongo`, `agent-slack`,
`agent-sql`, `lin`). The original TypeScript implementation was retired at the
v0.7.0 cutover — see git history and `design-docs/go-rewrite.md`.

## Architecture

```
cmd/agent-notion/main.go   # entry; version stamped via ldflags
internal/
  cli/                     # cobra tree: root.go + one file per command group
  config/                  # config.json I/O (byte-compatible legacy shape)
  credential/              # token resolution, workspace CRUD, OAuth refresh
  auth/                    # Notion Desktop + browser token_v2 extraction
  oauth/                   # OAuth consumer: callback server, code exchange
  errors/                  # API failures → {error, fixable_by, hint}
  notion/                  # Backend interface + normalized types (snake_case)
    official/              # public REST API client + backend
    v3/                    # unofficial API: client, record map, transforms,
                           # operations, backend, AI domain
    markdown/              # blocks ↔ markdown
  mocknotion/              # fixture-driven fake of both API surfaces (tests)
  ids/, truncation/        # ID normalization; {field}Length truncation
```

## Key patterns

- Built on published `lib-agent-*` tags (no `replace`): `lib-agent-cli`
  (root, flags, creds, ConfigCommand, RequireConfirm, EmitItem),
  `lib-agent-output` (NDJSON, errors, WriteList), `lib-agent-mcp` (`mcp`).
- **DI for tests**: `rootDeps` constructor injection + the hidden
  `--base-url` flag point commands at `mocknotion` end-to-end. Command tests
  never stub internals — they drive HTTP.
- **Two backends**: official REST and v3 desktop-session, dispatched by
  workspace auth type; `--backend auto|official|v3` overrides. Backend
  errors classify inside `withBackend`/`withV3Client` — command bodies just
  `return err`.
- **Output contract**: NDJSON records on stdout with `@pagination`/`@meta`
  trailers (`--format json|yaml` for envelopes), `{error, fixable_by, hint}`
  on stderr, exit 1. Destructive ops gate on `--yes`.
- **Shared state**: config at `~/.config/agent-notion/config.json`, keychain
  service `app.paulie.agent-notion`, secrets replaced by `__KEYCHAIN__`.
  The on-disk shapes are a compatibility contract — don't change them.
- **Never print tokens.** `auth status` reports source/workspace, not keys.

## Development

- `make build` / `make test` / `make vet` / `make lint` / `make dev ARGS="..."`
- Tests isolate state: `XDG_CONFIG_HOME=t.TempDir()`,
  `AGENT_NOTION_NO_KEYCHAIN=1`, fake keychains — never touch the real store.
  Fixtures use synthetic data only — never real IDs, content, or user data.

## Release

Run `/release <patch|minor|major>`: tag `vX.Y.Z` + push; the shared
`shhac/homebrew-tap` workflow cross-builds, publishes the GitHub release, and
updates the formula (needs the `TAP_DEPLOY_KEY` secret in the repo's
`homebrew-tap` environment).

## Keeping docs in sync

A change to commands/flags/output updates, in the same commit: the command's
usage card (`internal/cli/<group>.go`), `skills/agent-notion/`, and
`README.md`.
