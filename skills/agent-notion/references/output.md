# Output format (reference)

## General

All commands print JSON to stdout. Errors print `{ "error": "..." }` to stderr with non-zero exit.

Empty/null fields are pruned automatically — missing keys mean no value, not `null`.

## Truncation

Fields named `description`, `body`, or `content` are truncated to ~200 characters by default. A companion `*Length` field (e.g. `descriptionLength`) always shows the full character count.

**Default (truncated):**

```json
{
  "description": "This is the beginning of a long database description that goes on for many paragraphs...",
  "descriptionLength": 1847
}
```

**With `--full` or `--expand description` (expanded):**

```json
{
  "description": "This is the beginning of a long database description that goes on for many paragraphs and includes detailed specifications...",
  "descriptionLength": 1847
}
```

The `*Length` field is always present when the source field has content, regardless of truncation. Use it to detect whether content was truncated (`description.length < descriptionLength`).

Truncatable fields: `description`, `body`, `content`. Global flags: `--expand <field,...>` or `--full`.

## List output

List commands return:

```json
{
  "items": [ ... ],
  "pagination": {
    "hasMore": true,
    "nextCursor": "abc123"
  }
}
```

When there are no more pages, the `pagination` key is omitted entirely.

## Single item output

Single-item commands (e.g., `page get`, `user me`) return the object directly:

```json
{
  "id": "...",
  "url": "...",
  "properties": { ... }
}
```

## Search results (`search`)

```json
{
  "items": [
    {
      "id": "aaaaaaaa-1111-2222-3333-444444444444",
      "type": "page",
      "title": "Meeting Notes",
      "url": "https://www.notion.so/...",
      "parent": { "type": "database", "id": "bbbbbbbb-..." },
      "lastEditedAt": "2026-01-15T10:30:00.000Z"
    },
    {
      "id": "cccccccc-...",
      "type": "database",
      "title": "Project Tracker",
      "url": "https://www.notion.so/...",
      "parent": { "type": "workspace" },
      "lastEditedAt": "2026-01-14T08:00:00.000Z"
    }
  ],
  "pagination": { "hasMore": true, "nextCursor": "..." }
}
```

Parent types: `database`, `page`, `workspace`. The `id` field is only present for `database` and `page` parents.

## Database list items (`database list`)

```json
{
  "id": "...",
  "title": "Project Tracker",
  "url": "https://www.notion.so/...",
  "parent": { "type": "page", "id": "..." },
  "propertyCount": 8,
  "lastEditedAt": "2026-01-15T10:30:00.000Z"
}
```

## Database detail (`database get`)

Includes full property definitions with type-specific metadata:

```json
{
  "id": "...",
  "title": "Project Tracker",
  "description": "All active projects",
  "url": "https://www.notion.so/...",
  "parent": { "type": "page", "id": "..." },
  "properties": {
    "Name": { "id": "title", "type": "title" },
    "Status": {
      "id": "abc",
      "type": "status",
      "options": [
        { "name": "Not started", "color": "default" },
        { "name": "In Progress", "color": "blue" },
        { "name": "Done", "color": "green" }
      ],
      "groups": [
        { "name": "To-do", "options": ["Not started"] },
        { "name": "In progress", "options": ["In Progress"] },
        { "name": "Complete", "options": ["Done"] }
      ]
    },
    "Priority": {
      "id": "def",
      "type": "select",
      "options": [
        { "name": "High", "color": "red" },
        { "name": "Medium", "color": "yellow" },
        { "name": "Low", "color": "gray" }
      ]
    },
    "Tags": {
      "id": "ghi",
      "type": "multi_select",
      "options": [{ "name": "Frontend", "color": "blue" }]
    },
    "Task ID": { "id": "jkl", "type": "unique_id", "prefix": "TASK" },
    "Related": { "id": "mno", "type": "relation", "relatedDatabase": "..." }
  },
  "isInline": false,
  "createdAt": "2026-01-01T00:00:00.000Z",
  "lastEditedAt": "2026-01-15T10:30:00.000Z"
}
```

## Database schema (`database schema`)

Compact LLM-friendly format:

```json
{
  "id": "...",
  "title": "Project Tracker",
  "properties": [
    { "name": "Name", "id": "title", "type": "title" },
    { "name": "Status", "id": "abc", "type": "status", "options": ["Not started", "In Progress", "Done"], "groups": { "To-do": ["Not started"], "In progress": ["In Progress"], "Complete": ["Done"] } },
    { "name": "Priority", "id": "def", "type": "select", "options": ["High", "Medium", "Low"] },
    { "name": "Tags", "id": "ghi", "type": "multi_select", "options": ["Frontend"] },
    { "name": "Task ID", "id": "jkl", "type": "unique_id", "prefix": "TASK" },
    { "name": "Related", "id": "mno", "type": "relation", "relatedDatabase": "..." }
  ]
}
```

## Database query results (`database query`)

```json
{
  "items": [
    {
      "id": "...",
      "url": "https://www.notion.so/...",
      "properties": {
        "Name": "Fix login redirect",
        "Status": "In Progress",
        "Priority": "High",
        "Tags": ["Frontend", "Bug"],
        "Assignee": [{ "id": "...", "name": "Alice" }],
        "Due Date": { "start": "2026-02-01", "end": null },
        "Done": false,
        "Task ID": "TASK-42"
      },
      "createdAt": "2026-01-10T09:00:00.000Z",
      "lastEditedAt": "2026-01-15T10:30:00.000Z"
    }
  ],
  "pagination": { "hasMore": false }
}
```

Properties are flattened to simple values. See "Flattened property types" below.

## Page detail (`page get`)

```json
{
  "id": "...",
  "url": "https://www.notion.so/...",
  "parent": { "type": "database", "id": "..." },
  "properties": {
    "Name": "Meeting Notes",
    "Status": "Done",
    "Tags": ["Design"]
  },
  "icon": { "type": "emoji", "emoji": "📝" },
  "createdAt": "2026-01-10T09:00:00.000Z",
  "createdBy": { "id": "...", "name": "Alice" },
  "lastEditedAt": "2026-01-15T10:30:00.000Z",
  "lastEditedBy": { "id": "...", "name": "Bob" },
  "archived": false
}
```

### With `--content` (markdown)

Adds:

```json
{
  "content": "## Overview\n\nThis document covers...\n\n- Item 1\n- Item 2\n\n### Details\n\nMore text here.",
  "blockCount": 15,
  "contentTruncated": true
}
```

`contentTruncated` appears only when the page has more than 1000 blocks.

### With `--raw-content` (structured blocks)

Adds:

```json
{
  "blocks": [
    { "id": "...", "type": "heading_2", "content": "Overview", "hasChildren": false },
    { "id": "...", "type": "paragraph", "content": "This document covers...", "hasChildren": false },
    { "id": "...", "type": "bulleted_list_item", "content": "Item 1", "hasChildren": false }
  ],
  "blockCount": 15,
  "contentTruncated": true
}
```

## Page create (`page create`)

```json
{
  "id": "...",
  "url": "https://www.notion.so/...",
  "title": "New Page",
  "parent": { "database_id": "..." },
  "createdAt": "2026-01-15T10:30:00.000Z"
}
```

## Page update (`page update`)

```json
{
  "id": "...",
  "url": "https://www.notion.so/...",
  "lastEditedAt": "2026-01-15T10:30:00.000Z"
}
```

## Page archive (`page archive`)

```json
{
  "id": "...",
  "archived": true
}
```

## Block list — markdown mode (`block list`)

```json
{
  "pageId": "...",
  "content": "## Heading\n\nParagraph text\n\n- List item 1\n- List item 2",
  "blockCount": 4,
  "hasMore": false
}
```

## Block list — raw mode (`block list --raw`)

```json
{
  "items": [
    { "id": "...", "type": "heading_2", "content": "Heading", "hasChildren": false },
    { "id": "...", "type": "paragraph", "content": "Paragraph text", "hasChildren": false },
    { "id": "...", "type": "bulleted_list_item", "content": "List item 1", "hasChildren": false }
  ],
  "pagination": { "hasMore": true, "nextCursor": "..." }
}
```

## Block append (`block append`)

```json
{
  "pageId": "...",
  "blocksAdded": 3
}
```

## Comment list items (`comment list`)

```json
{
  "id": "...",
  "body": "This looks good!",
  "author": { "id": "...", "name": "Alice" },
  "createdAt": "2026-01-15T10:30:00.000Z"
}
```

`author` is `null` for bot-created comments without a user context.

## Comment add (`comment add`)

```json
{
  "id": "...",
  "body": "This looks good!",
  "createdAt": "2026-01-15T10:30:00.000Z"
}
```

## User list items (`user list`)

```json
{
  "id": "...",
  "name": "Alice Example",
  "type": "person",
  "email": "alice@example.com",
  "avatarUrl": "https://..."
}
```

`email` is only present for `person` type users.

## User me (`user me`)

```json
{
  "id": "...",
  "name": "My Integration",
  "type": "bot",
  "workspaceName": "Acme Corp"
}
```

## Auth status (`auth status`)

**Authenticated:**

```json
{
  "authenticated": true,
  "source": "keychain",
  "user": { "id": "...", "name": "My Bot", "type": "bot" },
  "workspace": {
    "alias": "acme",
    "name": "Acme Corp",
    "id": "...",
    "auth_type": "oauth"
  },
  "other_workspaces": [
    { "alias": "personal", "name": "Personal", "auth_type": "internal_integration" }
  ],
  "oauth_configured": true
}
```

**Not authenticated:**

```json
{
  "authenticated": false,
  "oauth_configured": false,
  "hint": "Run 'agent-notion auth setup-oauth' to configure OAuth, or 'agent-notion auth login --token <token>' for internal integrations."
}
```

## Auth login (`auth login`)

```json
{
  "ok": true,
  "workspace": {
    "alias": "acme",
    "name": "Acme Corp",
    "id": "...",
    "bot_id": "...",
    "default": true
  },
  "hint": "Add more workspaces with 'agent-notion auth login --alias <name>'"
}
```

## Auth workspace list (`auth workspace list`)

```json
{
  "items": [
    { "alias": "acme", "name": "Acme Corp", "auth_type": "oauth", "default": true },
    { "alias": "personal", "name": "Personal", "auth_type": "internal_integration" }
  ]
}
```

## Auth logout (`auth logout`)

```json
{
  "ok": true,
  "removed": "acme",
  "remaining_workspaces": ["personal"],
  "default_workspace": "personal"
}
```

## Config list-keys (`config list-keys`)

```json
{
  "keys": [
    {
      "key": "truncation.maxLength",
      "description": "Max characters before truncating description/body/content fields (default: 200, 0 = no truncation)",
      "default": 200
    },
    {
      "key": "pagination.defaultPageSize",
      "description": "Default number of results for list commands (default: 50, max: 100)",
      "default": 50
    }
  ]
}
```

## Config get/set (`config get`, `config set`)

```json
{
  "truncation.maxLength": 500
}
```

## Config reset (`config reset`)

```json
{
  "reset": "all"
}
```

Or for a single key:

```json
{
  "reset": "truncation.maxLength"
}
```

## Flattened property types

Page properties (from `page get` and `database query`) are flattened to simple values:

| Notion type        | Flattened output                                           |
| ------------------ | ---------------------------------------------------------- |
| title              | `"string"`                                                 |
| rich_text          | `"string"`                                                 |
| number             | `123` or `null`                                            |
| select             | `"Option Name"` or `null`                                  |
| multi_select       | `["Option1", "Option2"]`                                   |
| status             | `"Status Name"` or `null`                                  |
| date               | `{ "start": "2026-01-15", "end": null }` or `null`        |
| people             | `[{ "id": "...", "name": "Alice" }]`                       |
| checkbox           | `true` or `false`                                          |
| url                | `"https://..."` or `null`                                  |
| email              | `"alice@example.com"` or `null`                            |
| phone_number       | `"+1234567890"` or `null`                                  |
| files              | `[{ "name": "file.pdf", "url": "https://..." }]`          |
| relation           | `[{ "id": "..." }]`                                        |
| formula            | Result value (string, number, boolean, or date) or `null`  |
| rollup             | Recursively flattened values or `[]`                       |
| unique_id          | `"PREFIX-123"` or `"123"` (if no prefix)                   |
| created_time       | `"2026-01-15T10:30:00.000Z"` or `null`                    |
| last_edited_time   | `"2026-01-15T10:30:00.000Z"` or `null`                    |
| created_by         | `{ "id": "...", "name": "Alice" }` or `null`               |
| last_edited_by     | `{ "id": "...", "name": "Bob" }` or `null`                 |
| verification       | `"state"` or `null`                                        |

## Markdown block conversion

Block types converted when using `--content` or `block list` (without `--raw`):

| Block type           | Markdown output                    |
| -------------------- | ---------------------------------- |
| paragraph            | Plain text                         |
| heading_1/2/3        | `#` / `##` / `###`                |
| bulleted_list_item   | `- text`                           |
| numbered_list_item   | `1. text`                          |
| to_do                | `- [ ] text` or `- [x] text`      |
| toggle               | `> ▶ text`                         |
| code                 | Fenced code block with language    |
| quote                | `> text`                           |
| callout              | `> emoji text`                     |
| divider              | `---`                              |
| image                | `![caption](url)`                  |
| bookmark             | `[caption](url)`                   |
| equation             | `$$expression$$`                   |
| child_page           | `📄 Title`                         |
| child_database       | `📊 Title`                         |
| table_of_contents    | `[Table of Contents]`              |
| link_preview         | `[url](url)`                       |
| embed                | `[embed: url](url)`                |
| video/pdf/audio/file | `[type](url)` or `[name](url)`    |

Child blocks (nested content) are rendered with 2-space indentation.
