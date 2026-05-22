# V3 (`/api/v3/`) API surface — snapshot

> Captured 2026-02-17, with additions through 2026-05-22.
> Pinned client version at time of capture: `notion-client-version: 23.13.20260217.2221`.
> Source: HAR traces from the Notion web client + open-source v3 clients (react-notion-x, notion-py, kjk/notionapi, notion-enhancer).
>
> This is a point-in-time snapshot. Notion's v3 API is undocumented and unversioned — if you're acting on this in a new context, re-capture against the current client version before trusting any specific shape.

## Endpoint map

All endpoints use `POST https://www.notion.so/api/v3/{endpoint}` with JSON body, authenticated via `Cookie: token_v2={token}`.

### Read endpoints (as observed)

| Endpoint | Purpose | Request | Response |
|---|---|---|---|
| `getSpaces` | List all workspaces | `{}` | `{ [userId]: { space: { [spaceId]: { value: SpaceRecord } } } }` |
| `loadUserContent` | Top-level pages, user info, workspace data | `{}` | `{ recordMap: RecordMap }` |
| `loadPageChunk` | Page content (blocks, collections, signed URLs) | `{ pageId, limit, cursor, chunkNumber, verticalColumns }` | `{ recordMap: RecordMap }` |
| `loadCachedPageChunk` | Cached variant | Same as `loadPageChunk` | Same |
| `syncRecordValuesMain` | Fetch records by ID and table | `{ requests: [{ pointer: { id, table }, version }] }` | `{ recordMap }` |
| `getRecordValues` | Legacy record fetch (deprecated) | `{ requests: [{ id, table }] }` | `{ results: [{ value, role }] }` |
| `queryCollection` | Query database with filters/sorts/aggregations | `{ collectionId, collectionViewId, loader, query2 }` | `{ result: { blockIds, total, … }, recordMap }` |
| `search` | Full-text search across workspace | `{ type, query, ancestorId\|spaceId, limit, sort, filters }` | `{ results: [{ id, highlight, score }], total, recordMap }` |
| `getSignedFileUrls` | Authenticated file download URLs | `{ urls: [{ url, permissionRecord: { table, id } }] }` | `{ signedUrls: [string] }` |
| `getBacklinksForBlock` | Pages that reference a given block | `{ block: { id, spaceId } }` | `{ backlinks, recordMap }` |
| `getSnapshotsList` | Page version snapshots | `{ block: { id, spaceId }, size }` | `{ snapshots: [V3Snapshot] }` |
| `getActivityLog` | Workspace or per-page activity | `{ spaceId, navigableBlockId?, limit, startingAfterId? }` | `{ activityIds, activities, recordMap }` |
| `getPublicSpaceData` | Public workspace data | `{ spaceId }` | `{ results: [SpaceData] }` |

### Write endpoints (as observed)

| Endpoint | Purpose | Request shape |
|---|---|---|
| `saveTransactions` (older: `submitTransaction`) | Atomic write operations | `{ requestId, transactions: [{ id, spaceId, debug?, operations }] }` |
| `saveTransactionsFanout` | Same as `saveTransactions`, the variant the current desktop client posts to | Same |
| `deleteContentRecords` | Trash one or more records (or permanently delete) | `{ records: [{ id, table, spaceId }], permanentlyDelete: boolean }` |
| `restoreRecord` | Restore a single trashed record | `{ pointer: { table, id, spaceId } }` |
| `enqueueTask` | Queue async tasks (export, etc.) | `{ task: { eventName, request } }` |
| `getTasks` | Poll task status | `{ taskIds: [string] }` |

### Auth / setup endpoints

| Endpoint | Purpose |
|---|---|
| `loginWithEmail` | Email/password login; returns `token_v2` |

### `saveTransactions` operation primitives

Operations are `{ pointer: { table, id, spaceId }, path: [...], command, args }`:

| `command` | Purpose |
|---|---|
| `set` | Create/replace a record |
| `update` | Shallow-merge args into the record at `path` |
| `listBefore` | Prepend `id` to the list at `path` |
| `listAfter` | Append (or insert after a specific id) |
| `listRemove` | Remove `id` from the list |
| `removeChild` | Remove a child from a parent's list (modern variant used by the live trash flow; carries `additionalUpdatedPointers`) |
| `insertText` | CRDT text insert |
| `deleteText` | CRDT text delete (range-based) |

### Known record tables

From `syncRecordValuesMain` / `getRecordValues`:

| Table | Purpose |
|---|---|
| `block` | All content blocks (pages, text, lists, …) |
| `collection` | Databases (schema, properties) |
| `collection_view` | Database views (table, board, calendar, …) |
| `notion_user` | User profiles |
| `user_root` | User root settings |
| `user_settings` | User preferences |
| `space` | Workspace records |
| `space_view` | Workspace view settings (per-user sidebar order, etc.) |
| `space_user` | User-workspace membership |
| `team` | Team records |
| `sidebar_section` | Sidebar organization |
| `integration` | Integration/bot definitions |
| `discussion` | Comment threads (parent-of-many `comment` records) |
| `comment` | Individual comments |

## Authentication

### `token_v2` cookie

A session cookie used by the Notion desktop/web app, authenticating all `/api/v3/` requests. As observed:

- **Format:** `v03:{5-part-JWE}`, JWE header `{"alg":"dir","kid":"production:token-v3:2024-11-07","enc":"A256CBC-HS512"}` — server-side encrypted.
- **Lifetime:** ~365 days from creation.
- **HTTP-only and Secure:** yes.
- **Rotation:** re-issued on session refresh by the desktop/web app.
- **Programmatic refresh:** not possible — depends on the desktop/web app re-issuing the cookie.
- **Expired-token detection:** most endpoints return `401`. **`search` was observed returning `200 OK` with empty results for an expired token.** See `decisions/2026-02-v3-token-expiry-handling.md` for how the CLI handles this.

### Extraction from the macOS desktop app

The desktop app is Electron-based and uses Chromium's cookie encryption.

- Cookie database: `~/Library/Application Support/Notion/Partitions/notion/Cookies` (SQLite).
- Keychain entry: `"Notion Safe Storage"` — passphrase for AES key derivation.

Decryption sketch (Python; the in-CLI implementation lives in `src/lib/desktop-token.ts`):

```python
import hashlib, sqlite3, subprocess, binascii, urllib.parse, os

# 1. Get passphrase from macOS Keychain
#    $ security find-generic-password -s "Notion Safe Storage" -g
passphrase = b'<base64-encoded-passphrase>'

# 2. Derive AES-128 key via PBKDF2
key = hashlib.pbkdf2_hmac('sha1', passphrase, b'saltysalt', 1003, dklen=16)

# 3. Read encrypted cookie from SQLite
db = os.path.expanduser('~/Library/Application Support/Notion/Partitions/notion/Cookies')
conn = sqlite3.connect(db)
encrypted = conn.execute("SELECT encrypted_value FROM cookies WHERE name='token_v2'").fetchone()[0]
conn.close()

# 4. Decrypt with openssl, strip "v10" prefix
encrypted_data = encrypted[3:]
key_hex = binascii.hexlify(key).decode()
iv_hex = binascii.hexlify(b' ' * 16).decode()  # IV = 16 space characters
proc = subprocess.run(
    ['openssl', 'enc', '-aes-128-cbc', '-d', '-K', key_hex, '-iv', iv_hex, '-nopad'],
    input=encrypted_data, capture_output=True,
)
raw = proc.stdout

# 5. Remove PKCS7 padding
pad_byte = raw[-1]
unpadded = raw[:-pad_byte]

# 6. Skip 32-byte SHA256 domain hash (cookie meta version >= 24)
token_encoded = unpadded[32:].decode('utf-8')

# 7. URL-decode
token = urllib.parse.unquote(token_encoded)
```

### Other useful cookies (observed)

| Cookie | Format | Purpose |
|---|---|---|
| `token_v2` | v03 JWE (~791 chars) | Session auth |
| `file_token` | v02 (~171 chars) | File access |
| `notion_user_id` | UUID (36 chars) | User identifier |
| `p_sync_session` | JSON (~214 chars) | Sync session |

### Per-request headers

```
Cookie: token_v2={token}
x-notion-active-user-header: {userId}      # required for multi-account
x-notion-space-id: {spaceId}               # required for some endpoints
notion-client-version: {pinned version}
notion-audit-log-platform: web
```

## Official vs v3 capability comparison (snapshot)

| Feature | Official (`api.notion.com/v1`) | v3 (`notion.so/api/v3`) |
|---|---|---|
| Search | Title-only | Full-text content |
| Page content | `blocks/{id}/children` per level | Entire tree via `loadPageChunk` |
| Database query | Filters and sorts | Filters, sorts, aggregations, grouping, views |
| Database views | Not accessible | Full `collection_view` access |
| Page create | `POST /v1/pages` | `saveTransactions` operations |
| Page archive (Trash) | `PATCH /v1/pages/{id}` with `archived: true` | `saveTransactions` (alive: false) or `deleteContentRecords` |
| Page archive (real Archive) | Not exposed | `saveTransactions` (set `archived_*` fields) |
| Comments | `GET/POST /v1/comments` | `discussion` / `comment` records via sync + saveTransactions |
| File URLs | Temporary URLs in responses | `getSignedFileUrls` for permanent access |
| Block types | Most supported; some "unsupported" | All block types |
| Auth | Integration token or OAuth | `token_v2` cookie |
| Stability | Versioned, documented | Unversioned, undocumented |
| CORS | Allowed | Blocked (first-party app only) |

## Data format differences

### Page/block representation

**Official:**

```json
{
  "object": "page",
  "id": "<uuid>",
  "created_time": "<iso8601>",
  "last_edited_time": "<iso8601>",
  "parent": { "type": "database_id", "database_id": "<uuid>" },
  "properties": {
    "Name": {
      "id": "title",
      "type": "title",
      "title": [{ "type": "text", "text": { "content": "<text>" }, "plain_text": "<text>" }]
    },
    "Status": {
      "id": "abc1",
      "type": "select",
      "select": { "id": "<uuid>", "name": "Done", "color": "green" }
    }
  }
}
```

**V3:**

```json
{
  "id": "<uuid>",
  "type": "page",
  "version": 42,
  "created_time": 1705312800000,
  "last_edited_time": 1705320000000,
  "parent_id": "<uuid>",
  "parent_table": "collection",
  "alive": true,
  "properties": {
    "title": [["<text>"]],
    "abc1": [["Done"]]
  },
  "content": ["<child-uuid-1>", "<child-uuid-2>"],
  "space_id": "<uuid>"
}
```

### Key schema differences

| Aspect | Official | V3 |
|---|---|---|
| Property names | Human-readable ("Name", "Status") | Internal IDs ("title", "abc1") |
| Property resolution | Names included in response | Must resolve via collection schema |
| Rich text | `[{ type, text: { content }, annotations }]` | `[["text", [["b"]]]]` (compact array form) |
| Timestamps | ISO 8601 strings | Unix milliseconds |
| Parent reference | `{ type: "database_id", database_id }` | `{ parent_id, parent_table }` |
| Block content | Separate `blocks/{id}/children` call | Inline `content: ["child-id-1", …]` |
| Collections | "databases" | "collections" (views are separate records) |
| Response shape | Single resource per endpoint | Denormalized `RecordMap` |
| Pagination | `{ has_more, next_cursor }` | Chunk-based `{ cursor: { stack: [...] } }` |

### V3 rich-text decoration codes

```
[["Hello "], ["bold", [["b"]]], [" world"], ["‣", [["u", "<user-uuid>"]]]]
```

| Code | Meaning |
|---|---|
| `b` | bold |
| `i` | italic |
| `s` | strikethrough |
| `c` | inline code |
| `_` | underline |
| `h` | highlight |
| `a` | link |
| `u` | user mention |
| `p` | page mention |
| `d` | date mention |

### Block types

Both APIs use the same block type strings: `page`, `text`, `header`, `sub_header`, `sub_sub_header`, `bulleted_list`, `numbered_list`, `to_do`, `toggle`, `quote`, `callout`, `code`, `divider`, `image`, `video`, `file`, `bookmark`, `embed`, `equation`, `table_of_contents`, `column_list`, `column`, `collection_view`, `collection_view_page`, etc.

The property structure within each type differs substantially between the two APIs.

## Integration token sidegrade (research notes)

**Question we asked:** can `token_v2` be used to programmatically retrieve an official integration token (`ntn_*`) without manual copy-paste?

**Findings (as tested 2026-02-17):**

| Endpoint | Exists | Outcome |
|---|---|---|
| `getIntegrationSecret` | Yes | Works for integrations the active user owns; rejects others with `"You do not have permission to edit this integration"` |
| `deleteIntegration` | Yes | Same ownership check |
| `getRecordValues` (table: `integration`) | Yes | Returns full record except the secret |
| `createIntegration` / `createBot` | No (404) | ~15 endpoint-name variations tried; none worked |

**Integration record shape (via `syncRecordValuesMain`):**

```json
{
  "id": "<uuid>",
  "version": 28,
  "name": "<name>",
  "type": "guest" | "external" | "internal",
  "status": "unpublished" | "published",
  "parent_id": "<owner-user-uuid>",
  "parent_table": "notion_user",
  "redirect_uris": [],
  "capabilities": {
    "read_comment": true,
    "read_content": true,
    "insert_comment": true,
    "insert_content": true,
    "update_content": true,
    "read_user_with_email": true,
    "read_user_without_email": true
  },
  "info": {
    "icon": "<url>",
    "email": "<creator-email>",
    "tagline": "...",
    "has_viewed_secret": true
  }
}
```

The `token`/`secret` is not in the record — only `getIntegrationSecret` returns it.

**Conclusion:** a true sidegrade is not feasible in a single automated step. The smallest workable flow is:

1. User manually creates an internal integration once at `notion.so/my-integrations`.
2. CLI extracts `token_v2`, calls `getIntegrationSecret` with the integration id, receives the `ntn_*` token, stores it in keychain.

This was not implemented — the CLI currently keeps the v3 session and the official-API token as independent credentials. Captured here for future reconsideration.

## Open-source projects using v3 (as observed)

| Project | Notes |
|---|---|
| [react-notion-x](https://github.com/NotionX/react-notion-x) / `notion-client` | TypeScript renderer + client; widely used in production Next.js blogs |
| [notion-py](https://github.com/jamalex/notion-py) | Python client (read + write); unmaintained since 2021 |
| [kjk/notionapi](https://github.com/kjk/notionapi) | Go client |
| [notion-enhancer](https://notion-enhancer.github.io) | Desktop modification |

## Stability track record (as of 2026-02)

- Core read endpoints (`loadPageChunk`, `queryCollection`, `search`) have been stable for ~5+ years.
- `getRecordValues` → `syncRecordValuesMain` was a non-breaking migration.
- `token_v2` format changed `v02` → `v03` JWE during 2024 — broke some hardcoded parsers.
- Cookie encryption meta version 24 added a domain hash prefix — broke some extractors.
- No versioning header. No CORS support.

Risk profile, qualitative: low for reads, moderate for writes, higher for auth-adjacent features (`getIntegrationSecret`, login flows).
