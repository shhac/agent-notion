# `agent-notion` command map (reference)

Run `agent-notion usage` for concise LLM-optimized docs.
Run `agent-notion <command> usage` for detailed per-command docs.

Output is NDJSON on stdout; errors are `{error, fixable_by, hint}` on stderr (exit 1). `◆` marks v3-only commands (need `auth import-desktop`). `--yes` marks destructive commands that refuse to run without it.

## Auth

- `agent-notion auth setup-oauth --client-id <id> --client-secret <secret>` — store OAuth app credentials
- `agent-notion auth login [--alias <name>] [--port <port>]` — OAuth browser flow (default port 9876, falls forward to 9885)
- `agent-notion auth import [--token <token>] [--alias <name>]` — store an internal-integration token (token from `--token` or stdin)
- `agent-notion auth logout [--all] [--workspace <alias>] --yes` — remove credentials (`--all` clears every workspace, OAuth config, and keychain entries)
- `agent-notion auth status` — show the resolved credential, active workspace, and token source (never prints tokens)
- `agent-notion auth workspace list` — list all stored workspaces
- `agent-notion auth workspace switch <alias>` — set the default workspace
- `agent-notion auth workspace set-default <alias>` — alias for switch
- `agent-notion auth workspace remove <alias> --yes` — remove a stored workspace
- `agent-notion auth import-desktop [--skip-validation]` ◆ — import token_v2 from the Notion Desktop app
- `agent-notion auth import-browser <browser> [--profile <p>]` ◆ — import token_v2 from a browser cookie store (chrome, brave, edge, arc, chromium, firefox, zen, safari)

## Search

- `agent-notion search query <query> [--filter <type>] [--limit <n>] [--cursor <cursor>]` — search pages and databases by title (type: `page` | `database`)

**Note:** Notion search is title-only. It does not match page content, comments, or property values.

## Database

- `agent-notion database list [--limit <n>] [--cursor <cursor>]` — list all databases (uses the search API)
- `agent-notion database get <database-id>` — full database metadata with property definitions and options
- `agent-notion database query <database-id> [--filter <json>] [--sort <json>] [--limit <n>] [--cursor <cursor>]` — query rows with Notion filter/sort objects
- `agent-notion database schema <database-id>` — compact LLM-friendly schema (property names, types, options)

## Page

- `agent-notion page get <page-id> [--content] [--raw-content]` — page properties, optionally with content as markdown (`--content`) or structured blocks (`--raw-content`); adds `block_count`/`content_truncated`
- `agent-notion page create --parent <id> --title <title> [--properties <json>] [--icon <emoji>]` — create a page (auto-detects database vs page parent)
- `agent-notion page update <page-id> [--title <title>] [--properties <json>] [--icon <emoji>]` — update page properties (at least one option required)
- `agent-notion page trash <page-id> --yes` — move a page to Trash (recoverable; works on every backend; destructive)
- `agent-notion page restore <page-id>` — restore a page from Trash
- `agent-notion page archive <page-id> --yes` ◆ — real Archive: hide from search, page stays alive (destructive; distinct from Trash)
- `agent-notion page unarchive <page-id>` ◆ — undo real Archive
- `agent-notion page backlinks <page-id>` ◆ — pages that link to a given page (deduplicated by page)
- `agent-notion page history <page-id> [--limit <n>]` ◆ — version-history snapshots (default limit 20)

## Block

- `agent-notion block list <page-id> [--raw] [--limit <n>] [--cursor <cursor>]` — page content as markdown (default; tables render as GitHub-flavored pipe tables) or structured blocks (`--raw`, paginated; raw default limit 100)
- `agent-notion block append <page-id> [--content <markdown>] [--blocks <json>]` — append content as markdown or Notion block objects (one required)
- `agent-notion block update <block-id> --content <text>` — replace a block's text content
- `agent-notion block delete <block-id> --yes` — delete a block (destructive)
- `agent-notion block move <block-id> [--parent <block-id>] [--after <block-id>]` ◆ — move a block; `--parent` for cross-parent moves (e.g. into a callout/toggle); omit `--after` for first position (preserves block ID)
- `agent-notion block replace <page-id> [--content <markdown>] [--blocks <json>] --yes` — delete all blocks on a page, then append new content (one content source required; destructive)

## Comment

- `agent-notion comment list <page-id> [--limit <n>] [--cursor <cursor>]` — list comments on a page or block
- `agent-notion comment page <page-id> <body>` — add a page-level comment
- `agent-notion comment inline <block-id> <body> --text <target> [--occurrence <n>]` ◆ — add an inline comment anchored to specific text (`--occurrence` selects the nth match, default 1)

## User

- `agent-notion user list [--limit <n>] [--cursor <cursor>]` — list workspace users
- `agent-notion user me` — the bot (integration) identity

## Export (v3) ◆

- `agent-notion export page <page-id> [--format <markdown|html>] [--recursive] [--output <path>] [--wait <seconds>]` — export a page (or page tree with `--recursive`) to a markdown/HTML zip (default wait 120s)
- `agent-notion export workspace [--format <markdown|html>] [--output <path>] [--wait <seconds>]` — export the entire workspace (default wait 600s)
- `agent-notion export poll <task-id> [--output <path>] [--wait <seconds>]` — resume/poll a queued export by task ID (printed on `--wait` timeout)

## Activity (v3) ◆

- `agent-notion activity log [--page <page-id>] [--limit <n>]` — recent workspace or page activity log (default limit 20)

## AI (v3) ◆

- `agent-notion ai model list [--raw]` — list available AI models (default: name, family, tier; `--raw`: full objects with codenames and disabled models)
- `agent-notion ai chat list [--limit <n>]` — list recent AI chat threads
- `agent-notion ai chat send <message> [--thread <thread-id>] [--model <model>] [--page <page-id>] [--no-search] [--read-only] [--stream]` — send a message to Notion AI. By default the AI keeps its document-editing tools (matching Notion), so a prompt can modify a page; pass `--read-only` to request ask/answer mode (asks Notion's backend to disable those tools — a server-side request, not a client-enforced guarantee). `--model` accepts a codename or display name. `--stream` writes response text incrementally to stderr; `--debug` (global) also dumps raw NDJSON events. The JSON result always goes to stdout.
- `agent-notion ai chat get <thread-id> [--raw]` — get thread content (messages and metadata); `--raw` returns raw thread-message records
- `agent-notion ai chat mark-read <thread-id>` — mark a chat thread as read

Model resolution: `--model` flag > `config ai.default_model` > API default.

## Config

- `agent-notion config get <key>` — show one setting `{key, value, set}`
- `agent-notion config set <key> <value>` — update a setting
- `agent-notion config unset <key>` — reset one key to its default
- `agent-notion config list` — list every key with value + description

## Usage

- `agent-notion usage` — LLM-optimized top-level overview
- `agent-notion <command> usage` — detailed per-command docs:
  - `agent-notion search usage`
  - `agent-notion database usage`
  - `agent-notion page usage`
  - `agent-notion block usage`
  - `agent-notion comment usage`
  - `agent-notion user usage`
  - `agent-notion export usage`
  - `agent-notion activity usage`
  - `agent-notion ai usage`
  - `agent-notion auth usage`
  - `agent-notion config usage`

## Global flags

| Flag                    | Description                                                              |
| ----------------------- | ----------------------------------------------------------------------- |
| `--backend <mode>`      | API backend: `auto` (default), `official`, or `v3`                       |
| `--format <fmt>`        | `json`/`yaml` wrap output in one pretty `{data: […]}` envelope; `jsonl` is the default NDJSON |
| `--expand <field,...>`  | Expand specific truncated fields (e.g. `--expand description,body`)       |
| `--full`                | Expand all truncated fields                                              |
| `--color <mode>`        | `auto` (default), `always`, or `never`                                   |
| `--timeout <ms>`        | Request timeout in milliseconds                                          |
| `--debug` / `-d`        | Log debug diagnostics to stderr                                          |
| `--version` / `-v`      | Print the version                                                        |

## Config keys

| Key                     | Default | Range         | Description                                                    |
| ----------------------- | ------- | ------------- | -------------------------------------------------------------- |
| `page_size`             | 50      | 1–100         | Default number of results for list commands                    |
| `max_depth`             | —       | positive int  | Max nesting depth when recursively fetching blocks (unset = no limit) |
| `truncation.max_length` | 200     | positive int  | Max characters before truncating description/body/content       |
| `ai.default_model`      | —       | —             | Default AI model codename (see `ai model list --raw`)          |

## Property value shortcuts (page create/update `--properties`)

| JSON type | Notion mapping                      | Example                    |
| --------- | ----------------------------------- | -------------------------- |
| string    | `{ select: { name: value } }`       | `"Status": "Done"`         |
| number    | `{ number: value }`                 | `"Priority": 3`            |
| boolean   | `{ checkbox: value }`               | `"Archived": true`         |
| array     | `{ multi_select: [{ name }...] }`   | `"Tags": ["a", "b"]`       |
| object    | Passed through as Notion API format | `"Date": { "date": {...}}` |
