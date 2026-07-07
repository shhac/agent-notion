# Output format (reference)

## General

Commands print **NDJSON** to stdout — one JSON record per line. A single-item command prints one line; a list command prints one line per item followed by a trailer line (`{"@pagination": …}`, `{"@meta": …}`, or `{"@total": n}`) when relevant.

`--format json` or `--format yaml` collapses everything into one pretty document with a `data` array and the trailers hoisted as `@`-keys; `--format jsonl` is the default NDJSON.

Errors print to stderr as `{ "error": "...", "fixable_by": "agent|human|retry", "hint": "..." }` with exit code 1. `fixable_by` tells an agent who can resolve it. Empty/null fields are pruned — a missing key means no value.

Field names are snake_case (e.g. `last_edited_at`, `page_id`). The one exception is the truncation companion `{field}Length` (see below).

## Truncation

Fields named `description`, `body`, or `content` are truncated to 200 characters (runes) by default. A companion `{field}Length` field (e.g. `descriptionLength`) always carries the full rune count.

**Default (truncated):**

```json
{ "description": "This is the beginning of a long database description...", "descriptionLength": 1847 }
```

**With `--full` or `--expand description` (expanded):** the field carries the full text; `descriptionLength` is unchanged.

Detect clipping with `len(description) < descriptionLength`. Truncatable fields: `description`, `body`, `content`. Global flags: `--expand <field,...>`, `--full`. Raise the default cap with `config set truncation.max_length <n>`.

## List output (NDJSON)

Default (NDJSON) — one record per item, then a pagination trailer when more remain:

```
{"id":"a1b2c3d4-...","type":"page","title":"Meeting Notes","url":"https://www.notion.so/..."}
{"id":"c5d6e7f8-...","type":"database","title":"Project Tracker","url":"https://www.notion.so/..."}
{"@pagination":{"has_more":true,"next_cursor":"abc123"}}
```

Under `--format json`:

```json
{ "data": [ { "id": "a1b2c3d4-..." }, { "id": "c5d6e7f8-..." } ], "@pagination": { "has_more": true, "next_cursor": "abc123" } }
```

When there are no more pages the pagination trailer is omitted. Paginate with `--limit <n>` (max 100) and `--cursor <token>`.

## Single item output

Single-item commands (`page get`, `user me`, `page create`, …) print the object directly on one line.

## Search results (`search query`)

One record per hit:

```json
{ "id": "a1b2c3d4-...", "type": "page", "title": "Meeting Notes", "url": "https://www.notion.so/...", "parent": { "type": "database", "id": "b2c3d4e5-..." }, "last_edited_at": "2026-01-15T10:30:00.000Z" }
```

`type`: `page` | `database`. `parent.type`: `database` | `page` | `workspace`; `parent.id` is present only for `database`/`page` parents.

## Database list items (`database list`)

```json
{ "id": "b2c3d4e5-...", "title": "Project Tracker", "url": "https://www.notion.so/...", "parent": { "type": "page", "id": "..." }, "property_count": 8, "last_edited_at": "2026-01-15T10:30:00.000Z" }
```

## Database detail (`database get`)

Full property definitions with type-specific metadata:

```json
{
  "id": "b2c3d4e5-...",
  "title": "Project Tracker",
  "description": "All active projects",
  "url": "https://www.notion.so/...",
  "parent": { "type": "page", "id": "..." },
  "properties": {
    "Name": { "id": "title", "type": "title" },
    "Status": {
      "id": "abc", "type": "status",
      "options": [ { "name": "Not started", "color": "default" }, { "name": "Done", "color": "green" } ],
      "groups": [ { "name": "To-do", "options": ["Not started"] }, { "name": "Complete", "options": ["Done"] } ]
    },
    "Priority": { "id": "def", "type": "select", "options": [ { "name": "High", "color": "red" } ] },
    "Task ID": { "id": "jkl", "type": "unique_id", "prefix": "TASK" },
    "Related": { "id": "mno", "type": "relation", "related_database": "..." }
  },
  "is_inline": false,
  "created_at": "2026-01-01T00:00:00.000Z",
  "last_edited_at": "2026-01-15T10:30:00.000Z"
}
```

## Database schema (`database schema`)

Compact LLM-friendly format:

```json
{
  "id": "b2c3d4e5-...",
  "title": "Project Tracker",
  "properties": [
    { "name": "Name", "id": "title", "type": "title" },
    { "name": "Status", "id": "abc", "type": "status", "options": ["Not started", "Done"], "groups": { "To-do": ["Not started"], "Complete": ["Done"] } },
    { "name": "Priority", "id": "def", "type": "select", "options": ["High", "Medium", "Low"] },
    { "name": "Task ID", "id": "jkl", "type": "unique_id", "prefix": "TASK" },
    { "name": "Related", "id": "mno", "type": "relation", "related_database": "..." }
  ]
}
```

## Database query results (`database query`)

One record per row (properties flattened — see "Flattened property types"):

```json
{
  "id": "c3d4e5f6-...",
  "url": "https://www.notion.so/...",
  "properties": {
    "Name": "Fix login redirect", "Status": "In Progress", "Priority": "High",
    "Tags": ["Frontend", "Bug"], "Assignee": [{ "id": "...", "name": "Alice" }],
    "Due Date": { "start": "2026-02-01", "end": null }, "Done": false, "Task ID": "TASK-42"
  },
  "created_at": "2026-01-10T09:00:00.000Z",
  "last_edited_at": "2026-01-15T10:30:00.000Z"
}
```

## Page detail (`page get`)

```json
{
  "id": "d4e5f6a7-...",
  "url": "https://www.notion.so/...",
  "parent": { "type": "database", "id": "..." },
  "properties": { "Name": "Meeting Notes", "Status": "Done", "Tags": ["Design"] },
  "icon": { "type": "emoji", "emoji": "📝" },
  "created_at": "2026-01-10T09:00:00.000Z",
  "created_by": { "id": "...", "name": "Alice" },
  "last_edited_at": "2026-01-15T10:30:00.000Z",
  "last_edited_by": { "id": "...", "name": "Bob" },
  "archived": false
}
```

### With `--content` (markdown)

Adds `content`, `block_count`, and `content_truncated` (present only when a page has more than 1000 blocks):

```json
{ "content": "## Overview\n\nThis document covers...\n\n- Item 1\n- Item 2", "block_count": 15, "content_truncated": true }
```

### With `--raw-content` (structured blocks)

Adds `blocks` (flattened block objects), `block_count`, and `content_truncated`:

```json
{ "blocks": [ { "id": "...", "type": "heading_2", "content": "Overview", "has_children": false }, { "id": "...", "type": "paragraph", "content": "This document covers...", "has_children": false } ], "block_count": 15 }
```

## Page mutations

```json
// page create
{ "id": "...", "url": "https://www.notion.so/...", "title": "New Page", "parent": { "database_id": "..." }, "created_at": "2026-01-15T10:30:00.000Z" }
// page update
{ "id": "...", "url": "https://www.notion.so/...", "last_edited_at": "2026-01-15T10:30:00.000Z" }
// page trash --yes  /  page restore
{ "id": "...", "trashed": true }
{ "id": "...", "trashed": false }
// page archive --yes  /  page unarchive   (v3)
{ "id": "...", "archived": true }
{ "id": "...", "archived": false }
```

## Blocks

```json
// block list  (markdown mode)
{ "page_id": "...", "content": "## Heading\n\nParagraph text\n\n- List item 1", "block_count": 4, "has_more": false }
```

`block list --raw` — one record per block, then a pagination trailer:

```
{"id":"...","type":"heading_2","content":"Heading","has_children":false}
{"id":"...","type":"paragraph","content":"Paragraph text","has_children":false}
{"@pagination":{"has_more":true,"next_cursor":"..."}}
```

```json
// block append
{ "page_id": "...", "blocks_added": 3 }
// block update
{ "id": "...", "last_edited_at": "2026-01-15T10:30:00.000Z" }
// block delete --yes
{ "id": "...", "deleted": true }
// block move  (v3)
{ "id": "...", "parent_id": "...", "after_id": "..." }
// block replace --yes
{ "page_id": "...", "blocks_deleted": 5, "blocks_added": 3 }
```

## Comments

`comment list` — one record per comment (`author` is omitted for bot comments with no user context):

```json
{ "id": "...", "body": "This looks good!", "author": { "id": "...", "name": "Alice" }, "created_at": "2026-01-15T10:30:00.000Z" }
```

```json
// comment page
{ "id": "...", "discussion_id": "...", "body": "This looks good!", "created_at": "2026-01-15T10:30:00.000Z" }
// comment inline  (v3) — adds anchor_text
{ "id": "...", "discussion_id": "...", "body": "Great point!", "created_at": "2026-01-15T10:30:00.000Z", "anchor_text": "target phrase" }
```

## Users

`user list` — one record per user (`email` only for `person` type):

```json
{ "id": "...", "name": "Alice Example", "type": "person", "email": "alice@example.com", "avatar_url": "https://..." }
```

```json
// user me
{ "id": "...", "name": "My Integration", "type": "bot", "workspace_name": "Acme Corp" }
```

## Export (v3)

```json
// export page
{ "exported": "/absolute/path/to/notion-export-1234567890.zip", "format": "markdown", "pages_exported": 15, "recursive": true }
// export workspace
{ "exported": "/absolute/path/...", "format": "markdown", "pages_exported": 250 }
// export poll
{ "exported": "/absolute/path/...", "pages_exported": 250 }
```

`exported` is the absolute zip path. Progress is written to stderr during polling; on `--wait` timeout the task ID is printed for `export poll`.

## Backlinks (`page backlinks`) — v3

One record per source page (deduplicated by `page_id`), then a total:

```
{"block_id":"...","page_id":"...","page_title":"Meeting Notes"}
{"block_id":"...","page_id":"...","page_title":"Project Plan"}
{"@total":2}
```

## History (`page history`) — v3

One record per snapshot, then a total:

```
{"id":"...","version":42,"last_version":40,"timestamp":"2026-01-15T10:30:00.000Z","authors":["user-id-1","user-id-2"]}
{"@total":20}
```

## Activity (`activity log`) — v3

One record per activity:

```json
{ "id": "...", "type": "page-edited", "page_id": "...", "page_title": "Meeting Notes", "authors": ["Alice", "Bob"], "edit_types": ["content-change"], "start_time": "2026-01-15T10:00:00.000Z", "end_time": "2026-01-15T10:30:00.000Z" }
```

## AI (v3)

`ai model list` — one record per active model; `--raw` prints one full model object per line (including codename and disabled models):

```json
{ "name": "GPT-5.2", "family": "openai", "tier": "intelligent" }
```

`ai chat list` — one record per thread, then a meta trailer:

```
{"id":"...","title":"Summarize recent projects","created_at":1737000000000,"updated_at":1737000600000,"type":"workflow"}
{"@meta":{"unread_thread_ids":["thread-id-1"],"has_more":false}}
```

```json
// ai chat get   (--raw returns raw thread-message records instead of parsed messages)
{ "title": "Summarize recent projects", "messages": [ { "id": "...", "role": "user", "content": "Summarize my recent projects", "created_at": "..." }, { "id": "...", "role": "assistant", "content": "Based on your workspace..." } ] }
// ai chat send
{ "thread_id": "a1b2c3d4-...", "response": "Based on your workspace...", "title": "Summarize recent projects", "model": "oatmeal-cookie", "tokens": { "input": 1250, "output": 340, "cached": 800 } }
// ai chat mark-read
{ "ok": true }
```

`title` is present for new threads (auto-generated). With `--stream`, response text streams to stderr; the JSON result still prints to stdout at the end.

## Auth

```json
// auth status  (authenticated)
{ "authenticated": true, "source": "keychain", "workspace": "acme", "auth_type": "oauth" }
// auth status  (no credential) -> stderr error
{ "error": "no Notion credential configured", "fixable_by": "human", "hint": "run 'agent-notion auth login', import a desktop token, or set NOTION_TOKEN" }
// auth login
{ "ok": true, "storage": "keychain", "workspace": { "alias": "acme", "name": "Acme Corp", "id": "...", "bot_id": "...", "default": true }, "hint": "add more workspaces with 'agent-notion auth login --alias <name>'" }
// auth import  (internal integration)
{ "ok": true, "storage": "keychain", "workspace": { "alias": "acme", "name": "Acme Corp", "id": "...", "auth_type": "internal_integration", "default": true } }
// auth import-desktop  (v3)
{ "ok": true, "storage": "config", "extracted_at": "2026-01-15T10:30:00.000Z", "user": "Alice Example", "email": "alice@example.com", "space": "Acme Corp", "space_id": "...", "source": { "path": "..." } }
// auth logout --yes
{ "ok": true, "removed": "acme", "remaining_workspaces": ["personal"], "default_workspace": "personal" }
// auth logout --all --yes
{ "ok": true, "cleared": "all" }
```

`auth workspace list` — one record per workspace: `{ alias, name, auth_type, default }`. `auth workspace switch`/`set-default` — `{ ok, default_workspace }`.

## Config

```json
// config get <key>
{ "key": "truncation.max_length", "value": "500", "set": true }
// config set <key> <value>
{ "set": "truncation.max_length", "value": "500" }
// config unset <key>
{ "unset": "truncation.max_length" }
```

`config list` — one record per key: `{ key, value, set, description }`. `value` is a string; `set` is `false` for a key left at its default. Keys: `page_size`, `max_depth`, `truncation.max_length`, `ai.default_model`.

## Flattened property types

Page properties (from `page get` and `database query`) are flattened to simple values:

| Notion type      | Flattened output                                    |
| ---------------- | --------------------------------------------------- |
| title            | `"string"`                                          |
| rich_text        | `"string"`                                          |
| number           | `123` or `null`                                     |
| select           | `"Option Name"` or `null`                           |
| multi_select     | `["Option1", "Option2"]`                            |
| status           | `"Status Name"` or `null`                           |
| date             | `{ "start": "2026-01-15", "end": null }` or `null`  |
| people           | `[{ "id": "...", "name": "Alice" }]`                |
| checkbox         | `true` or `false`                                   |
| url              | `"https://..."` or `null`                           |
| email            | `"alice@example.com"` or `null`                     |
| phone_number     | `"+1234567890"` or `null`                           |
| files            | `[{ "name": "file.pdf", "url": "https://..." }]`    |
| relation         | `[{ "id": "..." }]`                                 |
| formula          | Result value (string/number/boolean/date) or `null` |
| rollup           | Recursively flattened values or `[]`                |
| unique_id        | `"PREFIX-123"` or `"123"` (no prefix)               |
| created_time     | `"2026-01-15T10:30:00.000Z"` or `null`              |
| last_edited_time | `"2026-01-15T10:30:00.000Z"` or `null`              |
| created_by       | `{ "id": "...", "name": "Alice" }` or `null`        |
| last_edited_by   | `{ "id": "...", "name": "Bob" }` or `null`          |
| verification     | `"state"` or `null`                                 |

## Markdown block conversion

Block types converted with `--content` or `block list` (without `--raw`):

| Block type           | Markdown output                 |
| -------------------- | ------------------------------- |
| paragraph            | Plain text                      |
| heading_1/2/3        | `#` / `##` / `###`              |
| bulleted_list_item   | `- text`                        |
| numbered_list_item   | `1. text`                       |
| to_do                | `- [ ] text` or `- [x] text`    |
| toggle               | `> ▶ text`                      |
| code                 | Fenced code block with language |
| quote                | `> text`                        |
| callout              | `> emoji text`                  |
| divider              | `---`                           |
| image                | `![caption](url)`               |
| bookmark             | `[caption](url)`                |
| equation             | `$$expression$$`                |
| child_page           | `📄 Title`                      |
| child_database       | `📊 Title`                      |
| table_of_contents    | `[Table of Contents]`           |
| link_preview         | `[url](url)`                    |
| embed                | `[embed: url](url)`             |
| video/pdf/audio/file | `[type](url)` or `[name](url)`  |

Child blocks (nested content) are rendered with 2-space indentation.
