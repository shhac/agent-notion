---
name: agent-notion
description: |
  Notion CLI for humans and LLMs. Use when searching, reading, exporting, creating, updating, archiving, or commenting on Notion pages, databases, blocks, workspace content, backlinks, history, recent activity, auth, or Notion AI chats/models. Triggers: "notion", "notion page", "notion database", "notion search", "query notion", "notion block", "notion comment", "notion auth", "notion export", "notion backlinks", "notion history", "notion activity", "notion ai", "notion chat", "ai model".
when_to_use: |
  Use when the user asks to search Notion, query database rows, read page properties/content, mutate pages or blocks, add/list comments, export pages/workspaces, inspect backlinks/version history/activity, manage Notion auth, or chat with Notion AI.
allowed-tools: Bash(agent-notion *) Read Grep Glob
---

# Notion automation with `agent-notion`

`agent-notion` is a CLI binary installed on `$PATH`. Invoke it directly (e.g. `agent-notion search query "Project Plan"`).

**Output is NDJSON** — one JSON record per line on stdout. List commands print one record per item, then a trailing `{"@pagination": {...}}` (or `{"@meta": ...}` / `{"@total": n}`) line when there is more. Pass `--format json|yaml` to get one pretty `{ "data": [ … ] }` envelope instead; `--format jsonl` is the default NDJSON.

**Errors** go to stderr as `{ "error": "...", "fixable_by": "agent|human|retry", "hint": "..." }` with exit code 1. Tokens are never printed.

## Backends

Two API backends: the **official REST API** (integration tokens, OAuth) and the **v3 desktop-session API** (`auth import-desktop`/`import-browser`). `--backend auto` (the default) prefers a stored v3 session, else the official credential; force one with `--backend official` or `--backend v3`. These commands require the v3 session: `export`, `page backlinks`, `page history`, `activity log`, `page archive`/`unarchive`, `block move`, `comment inline`, and all `ai` commands.

## Quick start (auth)

**Option A: OAuth (recommended for full official-API access)**

```bash
agent-notion auth setup-oauth --client-id <id> --client-secret <secret>
agent-notion auth login                        # opens the browser for the OAuth flow
agent-notion auth status
```

**Option B: Internal integration token**

```bash
agent-notion auth import --token ntn_...        # or pipe the token via stdin
agent-notion auth status
```

**Option C: Desktop session (for v3 features)**

```bash
agent-notion auth import-desktop                       # reads token_v2 from the Notion Desktop app
agent-notion auth import-browser chrome                # or from a browser cookie store
```

`import-browser` supports: chrome, brave, edge, arc, chromium, firefox, zen, safari (`--profile <p>` to pick a profile).

Multiple workspaces are supported:

```bash
agent-notion auth login --alias work
agent-notion auth workspace list
agent-notion auth workspace switch <alias>
agent-notion auth workspace remove <alias> --yes       # destructive: needs --yes
agent-notion auth logout --yes                          # default workspace; --all wipes everything
```

## Searching

**Important: Notion search is title-only.** It does not search page content, comments, or property values. To search within content, use `database query <id>` with property filters, or `block list <page-id>` and grep the output.

```bash
agent-notion search query "meeting notes"
agent-notion search query "Q1 Plan" --filter database
agent-notion search query "design doc" --filter page --limit 5
```

Each hit is one NDJSON record `{id, type, title, url, parent?, last_edited_at?}`; a trailing `{"@pagination": {has_more, next_cursor}}` line appears when more remain (pass `next_cursor` back via `--cursor`).

## Databases

```bash
agent-notion database list                                          # via the search API
agent-notion database get <database-id>                             # full metadata + property definitions
agent-notion database schema <database-id>                          # compact schema (types, options) for LLMs
agent-notion database query <database-id>                           # rows (one NDJSON record each)
agent-notion database query <id> --filter '{"property":"Status","status":{"equals":"Done"}}'
agent-notion database query <id> --sort '[{"property":"Name","direction":"ascending"}]'
```

Use `database schema` to discover property names, types, and valid select/status options before building filters.

## Pages

```bash
agent-notion page get <page-id>                                     # properties only
agent-notion page get <page-id> --content                           # properties + markdown content
agent-notion page get <page-id> --raw-content                       # properties + structured block objects
agent-notion page create --parent <id> --title "New Page"           # auto-detects database vs page parent
agent-notion page create --parent <db-id> --title "Task" --properties '{"Status":"In Progress","Priority":"High"}'
agent-notion page create --parent <id> --title "Notes" --icon "📝"
agent-notion page update <page-id> --title "Updated Title"          # at least one of --title/--properties/--icon
agent-notion page update <page-id> --properties '{"Status":"Done"}' --icon "✅"
agent-notion page trash <page-id> --yes                             # move to Trash (recoverable; destructive)
agent-notion page restore <page-id>                                 # restore from Trash
agent-notion page archive <page-id> --yes                           # real Archive (v3-only; destructive)
agent-notion page unarchive <page-id>                               # undo real Archive (v3-only)
```

Archive and Trash are independent Notion states. Trash sets `alive=false` and is reachable on every backend; Archive hides the page from search, leaves it alive, and requires the v3 backend. A page can be in either, both, or neither state.

`--content`/`--raw-content` add `block_count` and `content_truncated` (`true` when a page has more than 1000 blocks). Property values in `--properties` are auto-converted: strings become select values, numbers become number properties, booleans become checkboxes, arrays become multi-select; pass Notion API format for complex types.

## Blocks (page content)

```bash
agent-notion block list <page-id>                                   # markdown (default): {page_id, content, block_count, has_more}
agent-notion block list <page-id> --raw                             # one NDJSON record per block {id, type, content, has_children}
agent-notion block append <page-id> --content "## New Section\n\nParagraph text"
agent-notion block append <page-id> --blocks '[{"type":"paragraph","paragraph":{"rich_text":[{"text":{"content":"Hello"}}]}}]'
agent-notion block update <block-id> --content "New text"           # replace a single block's text
agent-notion block delete <block-id> --yes                          # destructive: needs --yes
agent-notion block move <block-id> --after <other-block-id>         # reorder (v3)
agent-notion block move <block-id> --parent <callout-id>            # move into a container (v3)
agent-notion block replace <page-id> --content "# Fresh\n\nAll new" --yes   # delete all blocks, then append (destructive)
```

Markdown conversion supports headings, lists, todos, code fences, blockquotes, callouts, images, and dividers. Use `block list --raw` to get block IDs for `update`/`delete`. `block replace` deletes every existing block before appending, so it requires `--yes`.

## Comments

```bash
agent-notion comment list <page-id>
agent-notion comment page <page-id> "This looks good!"
agent-notion comment inline <block-id> "Great point!" --text "target phrase"       # v3
agent-notion comment inline <block-id> "Second one" --text "the" --occurrence 2    # v3
```

`comment list` emits one NDJSON record per comment `{id, body, author: {id, name}, created_at}`. `comment page` returns `{id, discussion_id, body, created_at}`; `comment inline` adds `anchor_text` and requires a v3 desktop session. Discussion threads are not supported (all comments are top-level); the API cannot edit or delete comments.

## v3 features (require `auth import-desktop`)

### Export

```bash
agent-notion export page <page-id>                             # export as a markdown zip
agent-notion export page <page-id> --format html --recursive   # export the page tree as HTML
agent-notion export workspace                                  # export the entire workspace
agent-notion export workspace --output my-backup.zip           # custom output path
agent-notion export poll <task-id> --output backup.zip         # resume a timed-out export
```

Options: `--format markdown|html`, `--recursive` (page only), `--output <path>`, `--wait <seconds>` (page default 120, workspace/poll default 600). Exports are asynchronous; the CLI polls until completion or the `--wait` timeout, then prints the task ID so you can resume with `export poll`. Progress is written to stderr. Output: `{exported, format, pages_exported, recursive?}` where `exported` is the absolute zip path.

### Backlinks

```bash
agent-notion page backlinks <page-id>                          # pages linking to this page (v3)
```

One NDJSON record per source page `{block_id, page_id, page_title}` (deduplicated by page), then `{"@total": n}`.

### History

```bash
agent-notion page history <page-id>                            # version snapshots (v3)
agent-notion page history <page-id> --limit 50
```

One NDJSON record per snapshot `{id, version, last_version, timestamp, authors}`, then `{"@total": n}`.

### Activity

```bash
agent-notion activity log                                      # workspace-wide activity (v3)
agent-notion activity log --page <page-id>                     # scoped to a page
agent-notion activity log --limit 50
```

One NDJSON record per activity `{id, type, page_id, page_title, authors, edit_types, start_time, end_time}`.

## Notion AI (requires `auth import-desktop`)

```bash
agent-notion ai model list                                         # active models (name, family, tier)
agent-notion ai model list --raw                                   # full model objects incl. codenames
agent-notion ai chat list [--limit 10]                             # recent threads
agent-notion ai chat get <thread-id>                               # thread messages
agent-notion ai chat send "Summarize my recent projects"           # new conversation
agent-notion ai chat send "Tell me more" --thread <id>             # continue a thread
agent-notion ai chat send "Explain this page" --page <page-id>     # with page context
agent-notion ai chat send "Quick question" --stream                # stream response text to stderr
agent-notion ai chat send "Hello" --model "GPT-5.2"                # specific model
agent-notion ai chat mark-read <thread-id>
```

`ai chat send` returns `{thread_id, response, title, model, tokens: {input, output, cached}}`. Model resolution: `--model` flag > config `ai.default_model` > API default; accepts codenames (e.g. `oatmeal-cookie`) or display names. With `--stream`, response text streams to stderr while the JSON result still goes to stdout.

## Users

```bash
agent-notion user list                                              # one NDJSON record per user {id, name, type, email?, avatar_url?}
agent-notion user me                                                # bot identity {id, name, type, workspace_name}
```

`type` is `person` or `bot`; `email` is only present for person users.

## Truncation

Fields named `description`, `body`, and `content` are truncated to 200 characters by default. A companion `{field}Length` field (e.g. `descriptionLength`) always carries the full rune count, so an agent can tell content was clipped. Raise the cap or expand fields:

```bash
agent-notion --full page get <page-id>                              # expand every truncatable field
agent-notion --expand description database get <id>                 # expand one field
agent-notion --expand description,content page get <id> --content   # expand several
agent-notion config set truncation.max_length 500                   # raise the default cap
```

`--expand`/`--full` are global flags (place them before or after the command).

## Global flags

- `--backend auto|official|v3` — pick the API backend (default `auto`)
- `--format json|yaml|jsonl` — `json`/`yaml` wrap output in one pretty `{data: […]}` envelope; `jsonl` is the default NDJSON
- `--expand <fields>` / `--full` — lift field truncation
- `--color auto|always|never`, `--timeout <ms>`, `--debug`, `--version`

## IDs

Commands accept Notion UUIDs with or without dashes (dashless IDs from URLs are normalized automatically):

- `a1b2c3d4-1111-2222-3333-444444444444`
- `a1b2c3d411112222333344444444444444`

## Pagination

List commands stream NDJSON records, then a `{"@pagination": {has_more, next_cursor}}` trailer when more results remain. Pass `--limit <n>` (max 100) and `--cursor <token>` to page. Under `--format json|yaml` the records and pagination are folded into one `{data: […], "@pagination": …}` envelope.

## Destructive commands (require `--yes`)

`page trash`, `page archive`, `block delete`, `block replace`, `auth logout`, and `auth workspace remove` change state and refuse to run without `--yes`; without it they return a `fixable_by: human` error describing what would happen. `page restore` and `page unarchive` are not gated.

## Per-command usage docs

Every group has a `usage` subcommand with detailed, LLM-optimized docs:

```bash
agent-notion usage                # top-level overview
agent-notion search usage
agent-notion database usage
agent-notion page usage
agent-notion block usage
agent-notion comment usage
agent-notion export usage
agent-notion activity usage
agent-notion ai usage
agent-notion user usage
agent-notion auth usage
agent-notion config usage
```

Run `agent-notion <group> usage` when you need deep detail on a domain before acting.

## Configuration

```bash
agent-notion config list                                            # every key with value + description
agent-notion config get <key>                                       # one setting {key, value, set}
agent-notion config set truncation.max_length 500                   # raise the truncation cap
agent-notion config set page_size 20                                # default results per list command (1-100)
agent-notion config set ai.default_model <codename>                 # default AI model
agent-notion config unset <key>                                     # reset one key to its default
```

Keys: `page_size`, `max_depth`, `truncation.max_length`, `ai.default_model`. Settings persist in `~/.config/agent-notion/config.json`.

## References

- [references/commands.md](references/commands.md): full command map + all flags
- [references/output.md](references/output.md): NDJSON output shapes + field details
