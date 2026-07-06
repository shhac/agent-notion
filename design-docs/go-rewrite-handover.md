# Go rewrite — handover

For an agent (or human) picking up the TypeScript→Go migration of agent-notion
with no prior context from the sessions that started it. Read this first, then
[go-rewrite.md](./go-rewrite.md) (rationale + target layout) and
[go-rewrite-tracker.md](./go-rewrite-tracker.md) (live checklist). Root
`CLAUDE.md` has the day-to-day conventions.

---

## 1. The one-paragraph situation

agent-notion is a Notion CLI, historically TypeScript/Bun. We are porting it to
Go on Paul's shared `lib-agent-*` family (the same stack as the sibling repos
`agent-mongo`, `agent-slack`, `agent-sql`, `lin`). The Go code is being built at
the **repo root** (`cmd/`, `internal/`); the original TS implementation was
moved wholesale into **`bun/`** and stays runnable as the golden reference until
parity is signed off, then it gets deleted. Work happens on branch
**`migrate-to-go`** (not pushed). All four up-front decisions are settled (see
§5). **Phases 0–2 are done** (scaffold, config/credentials, the full auth
surface incl. OAuth login and token refresh); Phases 3–7 remain. The tracker
is the live status — trust it over the snapshots in this doc.

## 2. What exists right now

Branch `migrate-to-go`, commits (newest first):

- `feat[go/auth]: import token_v2 from Notion Desktop and browsers`
- `feat[go]: scaffold Go CLI + credential import`
- `chore: move bun/TS implementation into bun/ ahead of Go rewrite`
- `docs[design]: plan the Go rewrite on lib-agent-* family`

Go packages implemented and green (`go build/vet/test ./...`, golangci-lint 0
issues):

| Package | What it does | Status |
|---|---|---|
| `cmd/agent-notion` | 6-line entry; `version` via ldflags | done |
| `internal/cli` | cobra root on `libcli.NewRoot`; `auth` group | root + auth only |
| `internal/config` | config.json I/O, byte-compatible with the TS shape | done |
| `internal/credential` | access-token resolution + v3 session store/resolve | done (no refresh) |
| `internal/auth` | Notion Desktop + browser cookie extraction (token_v2) | done |
| `internal/notion/v3` | **only** `getSpaces` validation + `ParseGetSpacesSession` | stub — one file |
| `internal/oauth` | callback server, authorize URL, code exchange + refresh | done |
| `internal/notion/official` | **only** the users/me token probe | stub — one file |

Commands wired: the full `auth` group (`status`, `setup-oauth`, `login`,
`import`, `logout`, `workspace list/switch/set-default/remove`,
`import-desktop`, `import-browser <browser>`) plus `usage` / `auth usage`.
Every other group below is still TODO.

## 3. Build / test / run

```bash
make build            # -> ./agent-notion (gitignored), version from git describe
make test             # go test ./... -count=1
make vet
make lint             # golangci-lint run ./... (v2 config; ST1005 disabled)
make dev ARGS="auth status"

# Tests isolate all state — they never touch the real config or keychain:
#   t.Setenv("XDG_CONFIG_HOME", t.TempDir()); t.Setenv("AGENT_NOTION_NO_KEYCHAIN","1")

# The TS reference still runs from bun/:
cd bun && bun test    # 473 tests; the spec to port from
```

Go 1.26. Deps are published `github.com/shhac/lib-agent-*` tags (no `replace`,
no `go.work`) — resolvable via the normal proxy. Plus `spf13/cobra`,
`modernc.org/sqlite` (pure-Go, cookie DBs), `golang.org/x/sys` (Windows DPAPI).

## 4. What's left (the whole point of this doc)

Ordered by the phases in the tracker. The command counts are the real TS
surface (`bun/src/cli/<group>/`).

### Phase 2 remainder — auth ✅ DONE
All landed: `auth login` (OAuth via `internal/oauth`), `auth setup-oauth`,
`auth logout`, `auth workspace list/switch/set-default/remove`, `auth import`
(token via --token or stdin, validated with `internal/notion/official`
users/me), and token refresh (`credential.RefreshAccessToken`/
`RefreshOrRecover` — remember to wire into the 401 retry path in Phase 4).
Workspace CRUD lives in `internal/credential/workspace.go`; the `usage` +
`auth usage` LLM cards follow the agent-slack `usage.go`/`usage_text.go`
pattern.

### Phase 3 — pure transforms (parity target; port from `bun/src/notion/`)
- `internal/notion/v3/recordmap` — port `record-map.ts` (types, `normalize…`,
  the unwrap invariant we just consolidated) + `transforms.ts` (rich text →
  plain/markdown, block-type map, property flattening) + `operations.ts`
  (saveTransactions op builders) + `comments.ts`.
- `internal/notion/markdown` — port `markdown.ts` (blocks → markdown). This +
  rich text is the **differential-parity** target: check output against the
  `bun/` binary on the gnarly cases (decorations, anchor text, nested blocks).
- `internal/ids` — port `ids.ts` (URL/UUID normalization).
- `internal/truncation` — port `truncation.ts` (`{field}Length` companions).
- Bring fixtures across from `bun/test/` verbatim (synthetic data only — repo
  rule: never real IDs/content in fixtures).

### Phase 4 — HTTP clients + a mock server
- `internal/notion/v3` client — finish it: `loadPageChunk`, `syncRecordValues`,
  `queryCollection`, `search`, `saveTransactions`, `loadUserContent`, export
  endpoints, activity, backlinks, snapshots, `postStream` (NDJSON) for `ai`.
  Normalize-at-boundary (port `post()` → `normalizeRecordMapResponse`).
  Dependency-inject an `http.Doer` so it's testable.
- `internal/notion/official` — the official API client (port `notion/official/`
  and `notion/client.ts`). Decide: hand-rolled REST or a Go SDK. TS used
  `@notionhq/client`.
- `internal/mocknotion` — fixture-driven fake API + canonical response builders
  (mirror agent-slack's `internal/mockslack`) so CLI tests run with no network.

### Phase 5 — backend + command surface (the bulk)
- `NotionBackend` interface (port `notion/interface.ts`) with the two
  implementations (official + v3), and the normalized types (`notion/types.ts`).
- The 9 remaining command groups, ~100 leaf commands. Split big families across
  files, one `registerX(root, globals)` per group (see `internal/cli/auth.go`
  as the template). Remaining groups and their leaves:
  - **page** (9): archive, backlinks, create, get, history, restore, trash, unarchive, update
  - **block** (6): append, delete, list, move, replace, update
  - **database** (4): get, list, query, schema
  - **ai** (5): chat-get, chat-list, chat-mark-read, chat-send, model-list
  - **comment** (2): add, list
  - **export** (3): page, poll, workspace
  - **user** (2): list, me
  - **activity** (1): log
  - **search** (1): query
  - **config** (4): get, list-keys, reset, set → use `cli.ConfigCommand` (the
    lib provides get/set/unset/list; map the settings registry to `ConfigKey`s).
- Port each group's LLM usage card (`bun/src/cli/<group>/usage.ts`) ~verbatim.
- `--yes` gate on destructive ops (delete/trash/archive) via
  `cli.RequireConfirm`/`AddConfirmFlag`.
- **Decide domain-field casing** (see §5, open item): match the TS payload keys
  or go snake_case like the framework fields. Do this once, up front.

### Phase 6 — MCP
- One line: `root.AddCommand(agentmcp.Command(root, …))` (`lib-agent-mcp`).
  Annotate groups: `Expose`/`ReadOnly` the data groups (page/block/database/
  search/user/comment/activity), `Skip` auth/config. `WithOAuthKeyringService`.

### Phase 7 — docs, release, cutover
- Rewrite the shipped skill (`skills/agent-notion/`), `README.md`, and usage
  text for the Go contract, in lockstep.
- `.github/workflows/release.yml` already delegates to the shared
  `shhac/homebrew-tap` `go-release.yml` — verify `formula_class: AgentNotion`
  is right and that a tag push builds. (CI workflow `ci.yml` already present.)
- Parity sign-off: integration suite against a real test page + side-by-side
  golden runs vs the `bun/` binary. Then **delete `bun/`** and tag `v0.7.0`.

## 5. Settled decisions (do not relitigate)

1. **Break the output contract freely.** Pre-1.0. Converge on family
   conventions: NDJSON records on stdout, `@pagination`/`@meta` lines,
   `{error, fixable_by, hint}` on stderr, `--format json|yaml|jsonl`. The
   concrete breaks vs the TS output are catalogued in the tracker
   ("Broken contracts"). Update skill/README in the same change.
2. **Version bump is minor: 0.6.0 → 0.7.0** at cutover. The 0.x line continues.
3. **Desktop/browser token extraction follows agent-slack**: pure-Go,
   cross-platform (macOS/Linux/Windows), already implemented.
4. **`ai chat` streaming**: true streaming on the CLI path; buffered under MCP's
   subprocess model is acceptable.

Still genuinely open (decide when you reach it, don't guess now):
- **Domain-field casing** (Phase 5): keep TS payload keys vs snake_case. Default
  to whatever `agent-mongo`/`agent-slack` do for their domain payloads.
- **Official API client**: hand-rolled REST vs a Go SDK.

## 6. Gotchas and things learned the hard way

- **Shared state with the TS binary is the migration's safety net.** Both read
  `~/.config/agent-notion/config.json` and keychain service
  `app.paulie.agent-notion` (accounts `access_token:<alias>`,
  `refresh_token:<alias>`, `v3:token_v2`, `oauth_client_secret`). Secrets in
  config are the sentinel `__KEYCHAIN__`. `creds.Store` writes are byte-identical
  to the TS (`MarshalIndent` + `\n` + 0600). **Don't change the on-disk shape**
  or you break coexistence — keep the `internal/config` JSON tags matching
  `bun/src/lib/config.ts` exactly.
- **`ParseGetSpacesSession` iterates Go maps** → nondeterministic "first space"
  when a user has multiple non-team spaces (team/enterprise is preferred
  deterministically). Fine for the common case; if it bites, sort or thread
  order through. Same caution applies anywhere you port TS code that relied on
  `Object.values()` insertion order.
- **Chromium meta-version ≥24**: decrypted cookie plaintext carries a 32-byte
  SHA-256(host) prefix. agent-slack hides this by regex-scanning for `xoxd-`;
  Notion's `token_v2` has no signature, so we read `meta.version` and strip 32
  bytes explicitly (`normalizeCookiePlaintext`). Keep that pattern for any new
  cookie work.
- **SQLite: use `modernc.org/sqlite` (pure Go), never a cgo driver** — the
  family ships static binaries. Snapshot locked DBs to temp before reading
  (`copySqliteForRead`).
- **Never print tokens.** `auth status` reports source/workspace only. Keep it
  that way for every auth command.
- **zsh cwd persists across Bash tool calls** in this harness — a `cd bun` leaks
  into later commands. Use absolute paths for anything destructive; a cleanup
  once ran in the wrong directory during this migration. (No harm done, caught
  immediately.)
- **Doc-lockstep rule** (from `CLAUDE.md`): a change to commands/flags/output
  updates the usage text, `skills/agent-notion/`, `README.md`, and the tracker
  in the same commit.

## 7. Reference implementations (read these, don't reinvent)

- **`../agent-mongo`** — the closest precedent: an actual TS/Bun CLI ported to
  Go on this stack. Its `design-docs/go-rewrite.md` (status COMPLETE) is the
  playbook we're following. Copy its structure for `internal/{output,errors}`
  shims, the command-registration idiom, `.golangci.yml`, and `release.yml`.
- **`../agent-slack`** — patterns for a large command surface, the cookie
  extraction we ported (`internal/auth`), and the mock-server testing approach
  (`internal/mockslack` → your `mocknotion`).
- **`lib-agent-*`** libraries (pin these tags): `lib-agent-cli` v0.18.0 (root,
  flags, `creds`, `xdg`, `dialog`, `ConfigCommand`, `RequireConfirm`),
  `lib-agent-output` v0.10.0 (NDJSON, `Error`/`FixableBy*`, prune, redact),
  `lib-agent-mcp` v0.23.1 (`Command(root)`), `lib-agent-oauth` v0.7.1 (only if
  you want `mcp --oauth local`), `lib-agent-keyring` v0.1.1 (via `creds`).
  `lib-agent-cli/examples/demo/main.go` is a full working reference of the API.

## 8. Recommended next step

Finish Phase 2 auth first — it's self-contained, high-value, and unblocks real
use: port `auth login` (OAuth) + token refresh, then `logout`/`workspace`.
Then start Phase 3 (pure transforms) because it's the parity-critical layer and
everything in Phase 5 depends on it. Keep the `bun/` binary open in a second
terminal to diff outputs as you port.
