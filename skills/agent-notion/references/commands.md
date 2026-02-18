# `agent-notion` command map (reference)

Run `agent-notion usage` for concise LLM-optimized docs.
Run `agent-notion <command> usage` for detailed per-command docs.

## Auth

- `agent-notion auth setup-oauth --client-id <id> --client-secret <secret>` — configure OAuth credentials
- `agent-notion auth login [--alias <name>] [--port <port>] [--token <token>]` — authenticate via OAuth browser flow or `--token` for internal integrations (default port: 9876)
- `agent-notion auth logout [--all] [--workspace <alias>]` — remove active workspace credentials (--all: clear everything)
- `agent-notion auth status` — show auth state, active workspace, token source
- `agent-notion auth workspace list` — list all stored workspaces
- `agent-notion auth workspace switch <alias>` — set default workspace
- `agent-notion auth workspace set-default <alias>` — alias for switch
- `agent-notion auth workspace remove <alias>` — remove a stored workspace
- `agent-notion auth import-desktop [--skip-validation]` — import session from Notion Desktop app (macOS only, required for v3 commands)

## Search

- `agent-notion search <query> [--filter <type>] [--limit <n>] [--cursor <cursor>]` — search pages and databases by title (type: "page" | "database")

**Note:** Notion search is title-only. It does not match page content, comments, or property values.

## Database

- `agent-notion database list [--limit <n>] [--cursor <cursor>]` — list all databases (uses search API)
- `agent-notion database get <database-id>` — full database metadata with property definitions and options
- `agent-notion database query <database-id> [--filter <json>] [--sort <json>] [--limit <n>] [--cursor <cursor>]` — query rows with Notion filter/sort objects
- `agent-notion database schema <database-id>` — compact LLM-friendly schema (property names, types, options)

## Page

- `agent-notion page get <page-id> [--content] [--raw-content]` — page properties, optionally with content as markdown (`--content`) or structured blocks (`--raw-content`)
- `agent-notion page create --parent <id> --title <title> [--properties <json>] [--icon <emoji>]` — create page (auto-detects database vs page parent)
- `agent-notion page update <page-id> [--title <title>] [--properties <json>] [--icon <emoji>]` — update page properties (at least one option required)
- `agent-notion page archive <page-id>` — archive a page

## Block

- `agent-notion block list <page-id> [--raw] [--limit <n>] [--cursor <cursor>]` — page content as markdown (default) or structured blocks (`--raw`, paginated)
- `agent-notion block append <page-id> [--content <markdown>] [--blocks <json>]` — append content as markdown or Notion block objects (one required)

## Comment

- `agent-notion comment list <page-id> [--limit <n>] [--cursor <cursor>]` — list comments on a page
- `agent-notion comment page <page-id> <body>` — add a page-level comment
- `agent-notion comment inline <block-id> <body> --text <target> [--occurrence <n>]` — add an inline comment anchored to specific text (v3, requires desktop session)

## User

- `agent-notion user list [--limit <n>] [--cursor <cursor>]` — list workspace users
- `agent-notion user me` — current authenticated user/bot

## Export (v3)

- `agent-notion export page <page-id> [--format <markdown|html>] [--recursive] [--output <path>] [--timeout <seconds>]` — export page (or page tree with `--recursive`) to markdown/HTML zip
- `agent-notion export workspace [--format <markdown|html>] [--output <path>] [--timeout <seconds>]` — export entire workspace

## Backlinks (v3)

- `agent-notion backlinks <page-id>` — find pages that link to a given page (deduplicated by page)

## History (v3)

- `agent-notion history <page-id> [--limit <n>]` — version history snapshots for a page (default limit: 20)

## Activity (v3)

- `agent-notion activity [--page <page-id>] [--limit <n>]` — recent workspace or page activity log (default limit: 20)

## Config

- `agent-notion config get [key]` — get a setting value (omit key to show all)
- `agent-notion config set <key> <value>` — set a config value
- `agent-notion config reset [key]` — reset to defaults (omit key to reset all)
- `agent-notion config list-keys` — list all valid keys with descriptions and defaults

## Usage

- `agent-notion usage` — LLM-optimized top-level docs (~1000 tokens)
- `agent-notion <command> usage` — detailed per-command docs:
  - `agent-notion search usage`
  - `agent-notion database usage`
  - `agent-notion page usage`
  - `agent-notion block usage`
  - `agent-notion comment usage`
  - `agent-notion user usage`
  - `agent-notion export usage`
  - `agent-notion backlinks usage`
  - `agent-notion history usage`
  - `agent-notion activity usage`
  - `agent-notion auth usage`
  - `agent-notion config usage`

## Global flags

| Flag                   | Description                                                         |
| ---------------------- | ------------------------------------------------------------------- |
| `--expand <field,...>` | Expand specific truncated fields (e.g. `--expand description,body`) |
| `--full`               | Expand all truncated fields                                         |

## Config keys

| Key                          | Default | Range | Description                                                   |
| ---------------------------- | ------- | ----- | ------------------------------------------------------------- |
| `truncation.maxLength`       | 200     | >= 0  | Max characters before truncating description/body/content (0 = no truncation) |
| `pagination.defaultPageSize` | 50      | 1-100 | Default number of results for list commands                   |

## Property value shortcuts (page create/update --properties)

| JSON type | Notion mapping                       | Example                    |
| --------- | ------------------------------------ | -------------------------- |
| string    | `{ select: { name: value } }`       | `"Status": "Done"`         |
| number    | `{ number: value }`                  | `"Priority": 3`            |
| boolean   | `{ checkbox: value }`                | `"Archived": true`         |
| array     | `{ multi_select: [{ name }...] }`   | `"Tags": ["a", "b"]`      |
| object    | Passed through as Notion API format  | `"Date": { "date": {...}}` |
