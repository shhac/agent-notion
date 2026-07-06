# Go rewrite — tracker

Companion to [go-rewrite.md](./go-rewrite.md) (rationale + target layout).
This file is the live checklist. Tick items as they land; keep the "Now / Next"
line current so anyone picking up mid-flight knows where things stand.

**Now:** Phase 0 (scaffold) + Phase 1 (credential import) in progress.
**Next:** Phase 2 (auth flows).

## Resolved decisions

- **Break the output contract** — converge on family conventions (NDJSON,
  structured errors). Pre-1.0, no compatibility burden.
- **Version 0.6.0 → 0.7.0** minor bump at cutover; 0.x line continues.
- **Desktop-token extraction** follows agent-slack: pure-Go, build-tagged,
  cross-platform (not TS's macOS-only).
- **`ai chat` streaming** buffered under MCP, true streaming on CLI.
- The TS/Bun code stays runnable under `bun/` as the golden reference until
  parity sign-off, then is deleted.

## Dependencies (pin to these tags)

| Module | Tag |
|---|---|
| github.com/shhac/lib-agent-cli | v0.18.0 |
| github.com/shhac/lib-agent-output | v0.10.0 |
| github.com/shhac/lib-agent-mcp | v0.23.1 |
| github.com/shhac/lib-agent-oauth | v0.7.1 (transitive via mcp; direct only if `mcp --oauth local`) |
| github.com/shhac/lib-agent-keyring | v0.1.1 (transitive via cli/creds) |
| github.com/spf13/cobra | v1.10.2 |
| Go | 1.26 |

No `replace` directives — published tags only.

## Shared state (enables side-by-side running)

Go binary reads what the TS binary already writes — no migration step:
- Config: `~/.config/agent-notion/config.json`
- Keychain service: `app.paulie.agent-notion` (macOS `security` CLI, raw values)
- Accounts: `access_token:<alias>`, `refresh_token:<alias>`

## Broken contracts (catalogue)

Consumers of the current TS output see these changes. All intended.

1. **Lists**: `{ "items": [...] }` single pretty object → bare NDJSON, one
   record per line, no envelope.
2. **Pagination**: inline `pagination: { hasMore, nextCursor }` → trailing
   `{"@pagination": {...}}` line; fields rename `hasMore`→`has_more`,
   `nextCursor`→`next_cursor`.
3. **Single resource**: always pretty-printed (`JSON.stringify(…, 2)`) →
   compact NDJSON by default; `--format json|yaml` for pretty.
4. **Errors**: `{ "error": "msg" }` → `{ error, fixable_by, hint? }` on stderr,
   `fixable_by ∈ agent|human|retry`.
5. **New persistent flags**: `--format`, `--color`, `--timeout`, `--debug`,
   `--expose` (additive, but changes default stdout shape).
6. **Field casing**: framework fields go snake_case. Domain payload fields
   (page properties, block content) — decide in Phase 5 whether to match TS
   shape or family snake_case; default to whatever agent-mongo/agent-slack do.

Not broken: prune-empty, truncation + `{field}Length` companions, exit code 1,
domain payload structure (the parity target).

## Phases

### Phase 0 — scaffold  ⬅ in progress
- [x] Move TS impl to `bun/`, keep it green (473 tests)
- [x] `design-docs/go-rewrite.md` + this tracker
- [ ] `go.mod` (module `github.com/shhac/agent-notion`, go 1.26) + deps
- [ ] `cmd/agent-notion/main.go` (~6 lines; `version` ldflags; `cli.Run`)
- [ ] `internal/cli/root.go` via `libcli.NewRoot` (default NDJSON, unknown hint)
- [ ] `internal/output/` shim over lib-agent-output
- [ ] `internal/errors/` `{error, fixable_by, hint}` classification skeleton
- [ ] `Makefile` (build w/ ldflags version from `git describe`, test, lint, dev, tidy)
- [ ] `.golangci.yml` (v2; disable ST1005 — error strings are LLM contract)
- [ ] `.github/workflows/ci.yml` (build, vet, test, golangci)
- [ ] `AGENTS.md` + `CLAUDE.md` symlink; doc-lockstep rule
- [ ] `go build ./...` + `go test ./...` green (even if trivial)

### Phase 1 — config + credentials  ⬅ in progress
- [ ] `internal/config/` — read/write `config.json`, settings registry w/ defaults
- [ ] `internal/credential/` — `creds.NewKeychain("app.paulie.agent-notion")`,
      `__KEYCHAIN__` sentinel, config fallback, resolution order
      (flag > env `NOTION_TOKEN`/`AGENT_NOTION_*` > config)
- [ ] Read existing TS-written keychain entries (`access_token:<alias>` etc.)
- [ ] `--form` secret entry via `libcli/dialog`
- [ ] atomic writes + advisory lock (MCP runs parallel subprocesses)
- [ ] `agent-notion config get/set/unset/list` via `cli.ConfigCommand`
- [ ] `agent-notion auth status` (reads creds, prints source, no secret)
- [ ] tests: keychain interface fake, `XDG_CONFIG_HOME=t.TempDir()`,
      `AGENT_NOTION_NO_KEYCHAIN=1`; verify it reads a fixture config.json

### Phase 2 — auth flows
- [ ] OAuth callback server (port TS `oauth-server.ts`)
- [ ] Desktop-token extraction, agent-slack pattern: source registry,
      Chromium cookie SQLite read, `decryptChromiumCookie` (PBKDF2 + AES-128-CBC),
      macOS keychain passphrase, cross-platform build tags
- [ ] token_v2 validation (`getSpaces` raw fetch → `parseGetSpacesSession`)
- [ ] workspace aliases
- [ ] `auth import-desktop`, `auth import`, `auth login` (OAuth), `auth logout`

### Phase 3 — pure transforms (parity)
- [ ] `internal/notion/v3/record-map`: types, normalize, unwrap invariant
- [ ] rich text → plain/markdown; block-type map; property flattening; ID normalize
- [ ] port TS fixtures verbatim; differential-check gnarly paths vs `bun/` binary
      (rich-text decorations, anchor text)
- [ ] truncation w/ `{field}Length` companions

### Phase 4 — HTTP clients + mocknotion
- [ ] v3 client (normalize-at-boundary, headers, timeout, DI `Doer`)
- [ ] official API client (hand-rolled REST or SDK — decide)
- [ ] NDJSON stream reader for `ai chat` (postStream)
- [ ] `internal/mocknotion` fixture server + canonical body builders
- [ ] export task enqueue/poll + binary download

### Phase 5 — backend + command surface
- [ ] `NotionBackend` interface + both implementations
- [ ] 12 groups (activity, ai, auth, block, comment, config, database, export,
      page, search, usage, user), big families split across files
- [ ] usage cards ported ~verbatim; `--yes` gates on destructive ops
- [ ] decide domain-field casing (see broken-contracts #6)

### Phase 6 — MCP
- [ ] `agentmcp.Command(root)`; Expose/ReadOnly data groups; Skip auth/config
- [ ] `WithOAuthKeyringService`, hidden flags

### Phase 7 — docs, release, cutover
- [ ] rewrite skill (`skills/agent-notion/`) + README + usage in lockstep
- [ ] `.github/workflows/release.yml` → `shhac/homebrew-tap` `go-release.yml`
      (`name: agent-notion`, `formula_class: AgentNotion`, `help_match: "Notion CLI"`)
- [ ] parity sign-off: integration suite + side-by-side golden runs vs `bun/`
- [ ] delete `bun/`; tag `v0.7.0`

## Parity checklist (before deleting bun/)

- [ ] every TS command has a Go equivalent (diff `bun/src/cli` groups vs Go)
- [ ] domain payloads match (page/block/database/comment shapes)
- [ ] error hints preserved (LLM-facing messages)
- [ ] integration suite passes against a real test page
- [ ] skill/README describe the Go contract
