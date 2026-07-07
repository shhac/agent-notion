# Go rewrite — tracker

Companion to [go-rewrite.md](./go-rewrite.md) (rationale + target layout).
This file is the live checklist. Tick items as they land; keep the "Now / Next"
line current so anyone picking up mid-flight knows where things stand.

**Now:** Phases 0–5 done + the structure-pass milestone (16 commits:
classification at the seams, op-builder intent params, official
passthrough layer deleted, interface shrunk 24→22, truncation wired
with --expand/--full, credential/token-placement single owners, v3 file
splits, auth-extraction test suite 24→64% coverage, dead-code sweep) +
Phase 7 docs (README + shipped skill rewritten for the Go contract).
**Next:** MCP (Phase 6) → release-workflow check → parity sign-off
(integration run against a real test page needs credentials — human
step) → delete `bun/` → tag v0.7.0.
Known limitation for owner decision: lib cli.ConfigCommand writes NDJSON
directly, so the `config` group ignores --format (a lib-agent-cli fix if
the family wants it honored). Differential golden runs vs bun/ also
still pend real credentials.

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

### Phase 0 — scaffold  ✅
- [x] Move TS impl to `bun/`, keep it green (473 tests)
- [x] `design-docs/go-rewrite.md` + this tracker
- [x] `go.mod` (module `github.com/shhac/agent-notion`, go 1.26) + deps
- [x] `cmd/agent-notion/main.go` (~6 lines; `version` ldflags; `cli.Run`)
- [x] `internal/cli/root.go` via `libcli.NewRoot` (default NDJSON, unknown hint)
- [x] ~~`internal/output/` shim~~ dropped — commands use lib-agent-output directly
- [x] ~~`internal/errors/` skeleton~~ dropped — `output.New/Wrap` + `FixableBy*` suffice
- [x] `Makefile` (build w/ ldflags version from `git describe`, test, lint, dev, tidy)
- [x] `.golangci.yml` (v2; disable ST1005 — error strings are LLM contract)
- [x] `.github/workflows/ci.yml` (build, vet, test, golangci)
- [ ] `AGENTS.md` + `CLAUDE.md` symlink; doc-lockstep rule (CLAUDE.md exists; AGENTS.md pending)
- [x] `go build ./...` + `go test ./...` green (even if trivial)

### Phase 1 — config + credentials  ✅ (two items deferred)
- [x] `internal/config/` — read/write `config.json`, settings registry w/ defaults
- [x] `internal/credential/` — `creds.NewKeychain("app.paulie.agent-notion")`,
      `__KEYCHAIN__` sentinel, config fallback, resolution order
      (flag > env `NOTION_TOKEN`/`AGENT_NOTION_*` > config)
- [x] Read existing TS-written keychain entries (`access_token:<alias>` etc.)
- [ ] `--form` secret entry via `libcli/dialog` (deferred — add with Phase 5 polish)
- [ ] atomic writes + advisory lock (MCP runs parallel subprocesses) (deferred to Phase 6)
- [x] `agent-notion config get/set/unset/list` via `cli.ConfigCommand`
      (keys: page_size, max_depth, truncation.max_length, ai.default_model;
      family surface replaces TS reset/list-keys; zero values are rejected —
      they can't round-trip omitempty fields — with unset as the reset path)
- [x] `agent-notion auth status` (reads creds, prints source, no secret)
- [x] tests: keychain interface fake, `XDG_CONFIG_HOME=t.TempDir()`,
      `AGENT_NOTION_NO_KEYCHAIN=1`; verify it reads a fixture config.json

### Phase 2 — auth flows  ✅
- [x] Desktop/browser cookie extraction, agent-slack pattern (`internal/auth`):
      source registry (chrome/brave/edge/arc/chromium/firefox/zen/safari),
      Chromium cookie SQLite read via modernc.org/sqlite, `decryptChromiumCBC`
      (PBKDF2 + AES-128-CBC), macOS keychain / Linux secret-tool / Windows
      DPAPI+GCM, Safari binarycookies, Firefox moz_cookies, meta-v24 prefix strip
- [x] token_v2 validation (`getSpaces` → `ParseGetSpacesSession`), v3 session
      storage (`credential.StoreV3Session`, keychain + config)
- [x] `auth import-desktop`, `auth import-browser <browser>` (+ `--profile`,
      `--skip-validation`, completion)
- [x] OAuth callback server + `auth login` (`internal/oauth`: ListenCallback
      binds before building the authorize URL so the redirect URI always
      matches; Exchange/Refresh against the token endpoint, injectable for tests)
- [x] `auth logout` (--all / --workspace, `--yes` gate), `auth workspace`
      list/switch/set-default/remove (`internal/credential/workspace.go` —
      keychain-or-config token placement, alias derivation, default reassignment)
- [x] `auth import` (paste token via --token or stdin; validated with a
      minimal `internal/notion/official` users/me probe)
- [x] `auth setup-oauth` (client id/secret; secret to keychain when available)
- [x] token refresh: `credential.RefreshAccessToken`/`RefreshOrRecover`
      (atomic keychain+config swap; parallel-process recovery; clears tokens
      on total failure) — wire into the 401 retry path when clients land (Phase 4)
- [x] `usage` + `auth usage` LLM cards (agent-slack pattern:
      `usage.go`/`usage_text.go`; doc-lockstep applies to `usage_text.go`)

### Phase 3 — pure transforms (parity)
- [x] `internal/notion/v3` record map: entity types, the unwrap invariant
      (normalization moved from a TS tree-walk to `Entry.UnmarshalJSON` — both
      wire formats decode to the normalized shape), typed lookup helpers
      (sorted-ID iteration replaces JS insertion order), `RichText`/`Segment`/
      `Decoration` with tuple-form JSON round-tripping
- [x] `internal/notion` (types.go): normalized types, **snake_case domain
      fields** (resolves broken-contracts #6 — family convention + Notion's
      own API casing; TS camelCase was a local invention)
- [x] `internal/notion/v3/operations.go`: saveTransactions op builders
      (explicit `now time.Time` instead of Date.now(); sorted property/format
      key order); pure comment logic in comments.go (CollectDiscussionIDs,
      BuildAnchorTextMap, FindOccurrence)
- [x] `internal/notion/markdown`: blocks → markdown + markdown → official
      block objects
- [x] `internal/ids` (Normalize), `internal/truncation` (Truncator with
      `{field}Length` companions; rune-based lengths, not UTF-16 units)
- [x] `internal/notion/v3/transforms.go`: property flattening, block
      normalization, decoration-range injection, reverse (write-direction)
      property builders, comment/anchor-text transforms
- [ ] differential-check gnarly paths vs `bun/` binary (rich-text decorations,
      anchor text) — after the v3 client exists (Phase 4)

### Phase 4 — HTTP clients + mocknotion
- [x] v3 client (`internal/notion/v3/client.go`): all non-AI endpoints;
      normalize-at-boundary via the RecordMap decode path; injectable
      HTTP/BaseURL; 30s/60s default timeouts only when ctx has no deadline;
      `HTTPError` wire-text contract; `newUUID` stubbable. AI endpoints
      (getAvailableModels/getInferenceTranscriptsForUser/
      markInferenceTranscriptSeen) deferred to the Phase 5 ai group.
- [x] official API client (`internal/notion/official/`): **decided:
      hand-rolled REST, no SDK** (family ships zero-dep static binaries).
      Notion-Version pinned "2022-06-28" (= @notionhq/client 2.3.0's default,
      the shape the transforms target). Full endpoint + transform coverage;
      the TS's four "requires v3 backend" dispatch stubs intentionally live in
      the backend layer, not the REST client.
- [x] NDJSON stream reader (`internal/notion/v3/ndjson.go`): PostStream
      returns the body; ParseNDJSON callback iterator (10 MiB line cap; no
      stream timeout — caller ctx governs)
- [x] `internal/mocknotion`: fixture server (mockslack pattern) serving both
      surfaces — v3 keyed by endpoint name, official by "METHOD path";
      sticky queues, body-conditional handlers, ExpectTokenV2/ExpectBearer
      auth gates, RawBody for NDJSON; canonical body builders (Entry,
      RecordMapBody, PageChunkBody, BlockEntity)
- [ ] export task binary download (with the `export` command group, Phase 5)

### Phase 5 — backend + command surface
- [x] `notion.Backend` interface (26 methods incl. the four v3-only ops:
      ArchivePage/UnarchivePage/MoveBlock/AddInlineComment) + the **v3
      implementation** (`internal/notion/v3/backend.go` + comment
      orchestration), tested HTTP-level against mocknotion
- [x] official backend implementation (guidance errors for the four v3-only
      ops) + backend factory (`internal/cli/backend.go`: TS dispatch order,
      --backend override, withBackend OAuth 401-refresh-retry)
- [x] family-alignment pass (from the audit): GlobalFlags/rootDeps DI,
      emitItem/printList funnel over libcli.EmitItem/output.WriteList,
      lib-agent-cli/yaml registration, hidden --base-url, --timeout wiring,
      internal/errors classification seam, self-registering usage cards
- [x] `search query` — the command-group exemplar (withBackend + Classify,
      printPaginated @pagination trailer, page_size setting actually wired)
- [ ] 12 groups (activity, ai, auth, block, comment, config, database, export,
      page, search, usage, user), big families split across files
- [ ] usage cards ported ~verbatim; `--yes` gates on destructive ops
- [ ] decide domain-field casing (see broken-contracts #6)

### Phase 6 — MCP
- [x] `agentmcp.Command(root)`; data groups exposed (search/page/block/
      database/comment/user/export/activity/ai), search/user/activity marked
      read-only, auth/config skipped — wiring pinned by TestMCPWiring
- [x] `WithOAuthKeyringService(app.paulie.agent-notion.mcp)`, --color hidden

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
