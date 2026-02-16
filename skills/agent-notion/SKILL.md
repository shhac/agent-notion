---
name: agent-notion
description: |
  Notion CLI for humans and LLMs. Use when:
  - Searching Notion pages or databases by title
  - Querying database rows with filters and sorts
  - Reading page properties and content (as markdown or structured blocks)
  - Creating, updating, or archiving pages
  - Reading or appending block content (markdown or JSON blocks)
  - Listing or adding comments on pages
  - Managing Notion OAuth or integration token auth
  Triggers: "notion page", "notion database", "notion search", "query notion", "notion block", "notion comment", "notion auth", "notion content", "search notion", "create notion page", "notion workspace"
---

# Notion automation with `agent-notion`

`agent-notion` is a CLI binary installed on `$PATH`. Invoke it directly (e.g. `agent-notion search "Project Plan"`).

All output is JSON to stdout. Errors go to stderr as `{ "error": "..." }` with non-zero exit.

## Quick start (auth)

**Option A: OAuth (recommended for full access)**

```bash
agent-notion auth setup-oauth --client-id <id> --client-secret <secret>
agent-notion auth login                        # opens browser for OAuth flow
agent-notion auth status
```

**Option B: Internal integration token**

```bash
agent-notion auth login --token ntn_...
agent-notion auth status
```

Multiple workspaces are supported:

```bash
agent-notion auth login --alias work
agent-notion auth workspace list
agent-notion auth workspace switch <alias>
agent-notion auth logout [--all]
agent-notion auth workspace remove <alias>
```

## Searching

**Important: Notion search is title-only.** It does not search page content, comments, or property values.

```bash
agent-notion search "meeting notes"
agent-notion search "Q1 Plan" --filter database
agent-notion search "design doc" --filter page --limit 5
```

## Databases

```bash
agent-notion database list                                          # uses search API
agent-notion database get <database-id>                             # full metadata + property definitions
agent-notion database schema <database-id>                          # compact schema for LLMs (types, options)
agent-notion database query <database-id>                           # all rows
agent-notion database query <id> --filter '{"property":"Status","select":{"equals":"Done"}}'
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
agent-notion page update <page-id> --title "Updated Title"
agent-notion page update <page-id> --properties '{"Status":"Done"}' --icon "✅"
agent-notion page archive <page-id>
```

Property values in `--properties` are auto-converted: strings become select values, numbers become number properties, booleans become checkboxes, arrays become multi-select. Pass Notion API format for complex types.

## Blocks (page content)

```bash
agent-notion block list <page-id>                                   # markdown (default)
agent-notion block list <page-id> --raw                             # structured block objects (paginated)
agent-notion block append <page-id> --content "## New Section\n\nParagraph text"
agent-notion block append <page-id> --blocks '[{"type":"paragraph","paragraph":{"rich_text":[{"text":{"content":"Hello"}}]}}]'
```

Markdown conversion supports: headings, lists, todos, code fences, blockquotes, dividers, and paragraphs.

## Comments

```bash
agent-notion comment list <page-id>
agent-notion comment add <page-id> "This looks good!"
```

## Users

```bash
agent-notion user list
agent-notion user me                                                # current authenticated user/bot
```

## Truncation

Long text fields (`description`, `body`, `content`) are truncated to ~200 characters by default. A companion `*Length` field (e.g. `descriptionLength`) always shows the full size.

To see full content, use `--expand` or `--full`:

```bash
agent-notion --full page get <page-id>                              # expand all fields
agent-notion --expand description database get <id>                 # expand specific field
agent-notion --expand description,content page get <id> --content   # expand multiple
```

These are global flags — place them before the command or after it.

## IDs

All commands accept Notion UUIDs (with or without dashes):

- `aaaaaaaa-1111-2222-3333-444444444444`
- `aaaaaaaa111122223333444444444444`

## Pagination

List commands return `{ "items": [...], "pagination"?: { "hasMore": true, "nextCursor": "..." } }`.

Use `--limit <n>` (max 100) and `--cursor <token>` to paginate.

## Per-command usage docs

Every command group has a `usage` subcommand with detailed, LLM-optimized docs:

```bash
agent-notion usage                # top-level overview (~1000 tokens)
agent-notion search --usage       # search command details
agent-notion database usage       # database commands
agent-notion page usage           # page commands
agent-notion block usage          # block commands
agent-notion comment usage        # comment commands
agent-notion user usage           # user commands
agent-notion auth usage           # auth + workspace management
agent-notion config usage         # CLI settings keys, defaults, validation
```

Use `agent-notion <command> usage` when you need deep detail on a specific domain before acting.

## Configuration

```bash
agent-notion config list-keys                                       # all keys with defaults
agent-notion config get [key]                                       # current value(s)
agent-notion config set truncation.maxLength 500                    # increase truncation limit
agent-notion config set pagination.defaultPageSize 20               # change default page size
agent-notion config reset [key]                                     # restore defaults
```

## References

- [references/commands.md](references/commands.md): full command map + all flags
- [references/output.md](references/output.md): JSON output shapes + field details
