# Go rewrite

Status: **PLANNED**

Convert agent-notion from TypeScript/Bun to Go on the `lib-agent-*` family,
following the play `agent-mongo` ran (see its `design-docs/go-rewrite.md`,
status COMPLETE) and the layering `agent-slack` used for its ground-up build.

## Why

Every sibling (`agent-mongo`, `agent-slack`, `agent-sql`, `lin`) is Go on
`lib-agent-*`. agent-notion is the TS/Bun outlier, hand-rolling what the
family libs now consolidate: XDG config, keychain storage, secret dialogs,
output contract, CLI scaffolding, MCP exposure. Gains: one toolchain across
the family, smaller static binaries, `agent-notion mcp` for free, shared
release automation.

## Scope

Current TS: ~10.3k src LOC, ~7.2k test LOC, 112 commands across 12 groups
(activity, ai, auth, block, comment, config, database, export, page, search,
usage, user). Distinctive complexity vs siblings:

- **Dual backend** behind a `NotionBackend` interface: official API
  (`@notionhq/client`) and v3 unofficial API (record-map normalization,
  transforms, saveTransactions writes). This seam is the architecture — keep it.
- **Auth surface**: Notion OAuth (local callback server), desktop token
  extraction (Chromium cookie SQLite + AES-128-CBC decrypt + macOS keychain
  passphrase), token_v2 validation, workspace aliases.
- **Pure transform layer**: v3 RecordMap → normalized types, rich text,
  block-type mapping, property flattening — the parity-test target
  (analog of agent-slack's `render`, which was differentially fuzzed).
- **Streaming**: `ai chat` consumes NDJSON streams (postStream).
- **Export**: task enqueue/poll + binary download.

## Target layout

```
cmd/agent-notion/main.go        # ~6 lines; version ldflags; cli.Run(version)
internal/
  cli/                          # cobra tree; one package/file set per group;
                                #   registerX(root, globals) closures; usage cards
  notion/                       # NotionBackend interface + shared normalized types
  notion/official/              # official API client (hand-rolled REST, DI Doer)
  notion/v3/                    # v3 client, record-map, transforms, comments, ops
  auth/                         # oauth callback server, desktop-token extraction
  config/                       # ~/.config/agent-notion/config.json + settings registry
  credential/                   # keychain via creds.NewKeychain + __KEYCHAIN__ sentinel
  output/                       # thin shim over lib-agent-output
  errors/                       # {error, fixable_by, hint} classification
  truncation/                   # LLM truncation with {field}Length companions
  mocknotion/                   # fixture-driven fake API + canonical body builders
  integration/                  # //go:build integration, real Notion page
skills/agent-notion/            # unchanged home; updated in lockstep
```

Dependencies: `lib-agent-cli` v0.18.0, `lib-agent-output` v0.10.0,
`lib-agent-mcp` v0.23.1 (pulls oauth/keyring), `spf13/cobra`, Go 1.26.
No `replace` directives — published tags only, like both siblings.

## Migration principles (inherited from agent-mongo's playbook)

1. **Red-green from TS tests.** Port the existing test expectations (963
   asserts) to Go table tests first, then implement to green. The freshly
   consolidated record-map/transforms suites are the spec.
2. **Shared state, zero migration.** TS already uses
   `~/.config/agent-notion/config.json` and keychain service
   `app.paulie.agent-notion` via the `security` CLI — byte-compatible with
   `lib-agent-keyring` on macOS. Both binaries coexist during migration.
3. **Family output contract, not byte parity.** Adopt NDJSON lists +
   `@pagination`/`@meta`, `{error, fixable_by, hint}` on stderr, `--format
   json|yaml|jsonl`. Byte parity is required only for domain behavior:
   record-map normalization, rich-text/markdown conversion, property
   flattening, truncation, ID normalization, error hints.
4. **TS stays runnable as golden reference** until parity sign-off, then is
   deleted in one commit.
5. **Feature parity, not creep.** Only `mcp` and `--format yaml` arrive new,
   because they fall out of the libs.

## Phases

Bottom-up by layer (agent-slack's sequencing), not command-by-command:

- **Phase 0 — scaffold.** go.mod, `cmd/`, `internal/cli/root.go` via
  `libcli.NewRoot`, Makefile (ldflags version from `git describe`), CI
  workflow, `.golangci.yml` (disable ST1005 — error strings are LLM-facing
  contract), AGENTS.md with the doc-lockstep rule + CLAUDE.md symlink.
- **Phase 1 — config + credentials.** Config/settings registry; credential
  store reading the existing keychain entries (`access_token:<alias>`,
  `refresh_token:<alias>`) and config.json; `--form` secret entry via
  `libcli/dialog`; fslock-style atomic writes (MCP runs parallel subprocesses).
- **Phase 2 — auth flows.** Port OAuth callback server; desktop-token
  extraction (cookie SQLite read + `security` passphrase + PBKDF2/AES —
  agent-slack's `decryptChromiumCookie` is this exact algorithm; macOS-only,
  so shelling out to `/usr/bin/sqlite3` avoids a cgo/sqlite dependency —
  decide at implementation).
- **Phase 3 — pure transforms with parity.** record-map (types, normalize,
  unwrap invariant), rich text, block normalization, property flattening,
  markdown rendering. Port fixtures verbatim; differential-check against the
  TS binary for the gnarly paths (rich-text decorations, anchor text).
- **Phase 4 — HTTP clients + mocknotion.** v3 client (normalize-at-boundary,
  timeout, headers), official client, NDJSON stream reader for ai chat,
  fixture server + canonical body builders (mirrors `mockslack`).
- **Phase 5 — backend + command surface.** `NotionBackend` with both
  implementations; then the 12 command groups, splitting big families across
  files; usage cards ported ~verbatim; `--yes` gates on destructive ops.
- **Phase 6 — MCP.** `agentmcp.Command(root)`; `Expose`/`ReadOnly` data
  groups; `Skip` auth/config; `WithOAuthKeyringService`.
- **Phase 7 — docs, release, cutover.** Skill/README/usage lockstep pass;
  `release.yml` caller to `shhac/homebrew-tap/.github/workflows/go-release.yml`
  (`name: agent-notion`, `formula_class: AgentNotion`, `help_match: "Notion
  CLI"`) — replaces the manual `/release` skill; parity sign-off (integration
  suite + side-by-side golden runs against a test page); delete TS; tag.

## Open questions

1. **Output contract break**: current consumers of the TS CLI's output shape
   (skills, any scripts) see NDJSON + structured errors instead. Planned as a
   major-feel minor (family precedent), skill updated in the same phase — OK?
2. **Versioning across the rewrite**: continue 0.x from 0.6.0, or reset?
   (agent-mongo continued its line.)
3. **Windows/Linux support for desktop-token extraction**: TS is macOS-only;
   keep macOS-only or add DPAPI/Secret Service like agent-slack did?
4. **`ai chat` streaming in MCP context**: subprocess model buffers output;
   acceptable, or keep streaming CLI-only?
