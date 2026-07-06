# agent-notion

Notion CLI for humans and LLMs. **Mid-migration from TypeScript/Bun to Go.**

- The Go CLI is being built at the repo root (`cmd/`, `internal/`).
- The original TS/Bun implementation lives under `bun/` and stays runnable as
  the golden reference until parity sign-off. Its own preferences still apply
  when working inside `bun/` (bun runtime, `bun test`, etc.).
- Migration plan and progress: `design-docs/go-rewrite.md` and
  `design-docs/go-rewrite-tracker.md`. Read these before adding Go code.

## Architecture (Go)

```
cmd/agent-notion/main.go   # entry; version stamped via ldflags
internal/
  cli/                     # cobra tree on lib-agent-cli (root.go, one file per group)
  config/                  # config.json I/O — byte-compatible with the TS shape
  credential/              # token resolution (env > keychain > config)
```

## Key patterns

- Built on the `lib-agent-*` family: `lib-agent-cli` (root, flags, creds, XDG,
  dialog), `lib-agent-output` (NDJSON contract, structured errors), later
  `lib-agent-mcp` (the `mcp` server). Pin published tags; no `replace`.
- **Shared state with the TS binary**: config at
  `~/.config/agent-notion/config.json`, keychain service
  `app.paulie.agent-notion`, secrets replaced by `__KEYCHAIN__` in config.
- **Output contract**: NDJSON records on stdout, `@pagination`/`@meta` lines,
  `{error, fixable_by, hint}` on stderr, exit 1 on failure. Commands `return`
  errors from `RunE`; only `libcli.Run` renders them.
- **Never print tokens.** `auth status` reports source/workspace, not the key.

## Development

- `make build` / `make test` / `make vet` / `make lint` / `make dev ARGS="..."`
- Tests isolate state: `XDG_CONFIG_HOME=t.TempDir()`,
  `AGENT_NOTION_NO_KEYCHAIN=1`, fake keychains — never touch the real store.

## Release

Push a `vX.Y.Z` tag; `.github/workflows/release.yml` delegates to the shared
`shhac/homebrew-tap` workflow (cross-build, GitHub release, formula update).

## Keeping docs in sync

A change to commands/flags/output updates, in the same commit: the command's
usage text, `skills/agent-notion/`, `README.md`, and the tracker.
