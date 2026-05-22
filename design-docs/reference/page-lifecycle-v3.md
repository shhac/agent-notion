# Notion Page Lifecycle — HAR Reference

> Status: Reference notes — `design-docs/` is gitignored.
> Source: a desktop-web HAR captured by exercising create → type → archive → type → unarchive → trash → restore on a single throwaway page. All IDs/timestamps in this doc are placeholders.

This doc captures what the Notion web client *actually does* for the page-lifecycle states so we can reproduce the calls from the v3 backend. It's the source of truth for #1 (real Archive) and a future revisit of the Trash op shape.

## State summary

| State    | `alive` | Distinguishing fields                                                   |
|----------|---------|-------------------------------------------------------------------------|
| Active   | `true`  | —                                                                       |
| Archived | `true`  | `archived_by_id`, `archived_by_table`, `archived_time`                  |
| Trashed  | `false` | `moved_to_trash_id`, `moved_to_trash_table`, `moved_to_trash_time`      |

Archive and Trash are independent. A page can be archived (`alive: true`, `archived_*` set) and then trashed (`alive: false`, both sets present). Restoring from trash does not touch the `archived_*` fields.

## Common request shape

Both `saveTransactionsFanout` and the other v3 POSTs share these headers:

```
Content-Type: application/json
Cookie: token_v2=<token>
notion-client-version: <e.g. 23.13.20260519.1354>
notion-audit-log-platform: web
x-notion-active-user-header: <userId>
x-notion-space-id: <spaceId>   (not on every endpoint)
```

`saveTransactionsFanout` wraps one or more transactions:

```json
{
  "requestId": "<uuid>",
  "transactions": [
    {
      "id": "<uuid>",
      "spaceId": "<spaceId>",
      "debug": { "userAction": "<sourceLabel>" },
      "operations": [ ... ]
    }
  ]
}
```

The `debug.userAction` string is purely informational (it's what shows up in audit logs); the server accepts any value. We can use it to tag our requests so audit logs are legible — e.g. `"agent-notion.page.archive"`.

## Operation primitives observed

| `command`     | `args` keys                                                              | Notes |
|---------------|--------------------------------------------------------------------------|-------|
| `set`         | full block (`id`, `type`, `properties`, `space_id`, `created_time`, …)   | Create record. |
| `update`      | partial field bag                                                        | Merge into record. |
| `listBefore`  | `{ id }`                                                                 | Prepend `id` to the list at `path`. |
| `listAfter`   | `{ id }` (optionally `{ id, after }`)                                    | Append/insert. |
| `removeChild` | `{ id }` + sibling `additionalUpdatedPointers`                            | Remove from a parent's list (used by trash, not by old archiveBlockOps). |
| `insertText`  | `{ type, textInstanceId, searchLabel, id, originId, content, prevItems }` | CRDT text insert. |
| `deleteText`  | `{ type, textInstanceId, searchLabel, idRanges }`                        | CRDT text delete. |

## Archive (real, Business/Enterprise Beta)

UI source: `archiveActions.archiveBlocksWithMaybeSubitems`. One `saveTransactionsFanout` request, two ops:

```json
{
  "operations": [
    {
      "pointer": { "table": "block", "id": "<pageId>", "spaceId": "<spaceId>" },
      "path": [],
      "command": "update",
      "args": {
        "archived_by_id": "<userId>",
        "archived_by_table": "notion_user",
        "archived_time": <ms>
      }
    },
    {
      "pointer": { "table": "block", "id": "<pageId>", "spaceId": "<spaceId>" },
      "path": [],
      "command": "update",
      "args": {
        "last_edited_time": <ms>,
        "last_edited_by_id": "<userId>",
        "last_edited_by_table": "notion_user"
      }
    }
  ]
}
```

No parent-list mutation. `alive` is untouched. The page remains addressable; Notion just renders an "Archived" banner and hides it from search.

## Unarchive

UI source: `ArchivedPageBanner.handleUnarchive`. Identical to archive, but the three `archived_*` fields are `null`:

```json
{
  "command": "update",
  "args": {
    "archived_by_id": null,
    "archived_by_table": null,
    "archived_time": null
  }
}
```

Plus the same `last_edited_*` bump.

## Trash (current desktop client behaviour)

The current `src/notion/v3/operations.ts:archiveBlockOps` uses the older `alive: false` + `listRemove` shape. The live client now does it as **two requests**:

### 1. `saveTransactionsFanout` — `removeChild` on the parent's list

UI source: `selectableBlockActions.removeBlocksWithMaybeConfirmation`.

```json
{
  "operations": [
    {
      "pointer": { "table": "<parentTable>", "id": "<parentId>", "spaceId": "<spaceId>" },
      "path": ["<listName>"],
      "command": "removeChild",
      "args": { "id": "<pageId>" },
      "additionalUpdatedPointers": [
        { "table": "block", "id": "<pageId>", "spaceId": "<spaceId>" }
      ]
    },
    {
      "pointer": { "table": "block", "id": "<pageId>", "spaceId": "<spaceId>" },
      "path": [],
      "command": "update",
      "args": {
        "last_edited_time": <ms>,
        "last_edited_by_id": "<userId>",
        "last_edited_by_table": "notion_user"
      }
    }
  ]
}
```

Parent list paths observed:
- Top-level page (parent is space): `pointer.table = "space"`, `path = ["pages"]`.
- Child page (parent is block): `pointer.table = "block"`, `path = ["content"]`.

`additionalUpdatedPointers` tells the server "also bump these records' versions in any subscribers' record map" — it's an optimisation hint, the server appears to tolerate it being omitted but the client always sends it for the trashed page.

### 2. REST `POST /api/v3/deleteContentRecords`

```json
{
  "records": [
    { "id": "<pageId>", "table": "block", "spaceId": "<spaceId>" }
  ],
  "permanentlyDelete": false
}
```

Server sets `alive: false` and all three `moved_to_trash_*` fields. The response is a full `recordMap` for the affected records reflecting the trashed state.

With `permanentlyDelete: true`, this is what bypasses Trash entirely — useful if we ever want a `page delete --permanent` verb. Not implemented today.

### Why two requests?

Best guess: the `removeChild` is the live UI mutation (so the sidebar updates immediately via local CRDT), and `deleteContentRecords` is what authoritatively marks the page as trashed and triggers cascading effects (search index removal, child-block trashing, audit log, etc). The old `alive: false` op the codebase currently uses still works because the server still accepts the legacy shape — but it skips whatever side effects `deleteContentRecords` triggers.

**If we ever migrate the trash op, the migration is:**

1. Replace `archiveBlockOps` (rename to `trashedRemoveChildOp` or similar) with the `removeChild` shape.
2. Add a `deleteContentRecords` helper on `V3HttpClient`.
3. In `V3Backend.trashPage`, fetch the parent (already do this — see `backend.ts:381` for parent lookup), pick `("space", "pages")` vs `("block", "content")` based on `parent_table`, send the saveTransactionsFanout, then call `deleteContentRecords`.

## Restore from Trash

Single dedicated REST endpoint — no `saveTransactions` involved:

```
POST /api/v3/restoreRecord
{ "pointer": { "table": "block", "id": "<pageId>", "spaceId": "<spaceId>" } }
```

This is dramatically simpler than reconstructing the parent-list position would be: Notion server-side remembers where the page lived and re-inserts it. The response is again a full `recordMap` with the restored state (`alive: true`, all `moved_to_trash_*` fields cleared).

## Page create (for reference)

UI source: `sidebarActions.quickAddPage.directMoveToAdd`. A single transaction with five ops:

1. `set` on the new block: full record (`id`, `type: "page"`, `properties.title: []`, `space_id`, `created_*`, `crdt_data`, `crdt_format_version: 1`).
2. `update` on the new block: `{ permissions: [{ type: "user_permission", role: "editor", user_id: "<userId>" }] }`.
3. `update` on the new block: `{ parent_id, parent_table, alive: true }`.
4. `listBefore` on the parent's list pointer with `{ id: "<pageId>" }` — for sidebar pages the pointer is `{ table: "space_view", id: "<spaceViewId>", path: ["private_pages"] }`, **not** the space directly.
5. `update` on the new block: standard `last_edited_*` bump.

Note the asymmetry with trash: creation writes into the user's `space_view.private_pages`, while trash removes from the space's authoritative `space.pages`. That distinction would matter if we ever build a "sidebar order" feature.

## Text editing (for reference)

CRDT-based — each keystroke or selection delete fires its own `saveTransactionsFanout` with an `insertText` / `deleteText` op plus a `last_edited_*` update.

- `insertText.args`: `type`, `textInstanceId` (per-block CRDT id, lives in `crdt_data`), `searchLabel`, `id` (CRDT element id — `[clientId, counter]`), `originId` (preceding CRDT element), `content` (the inserted string), `prevItems` (the visible CRDT items immediately before the insert, used for conflict resolution).
- `deleteText.args`: `type`, `textInstanceId`, `searchLabel`, `idRanges` (CRDT id ranges to tombstone).

We are **not** implementing CRDT text editing — block content writes go through `block append` / `block update` which use the simpler `properties.title` array-of-arrays shape. Documenting this here purely so a future reader understands why the Notion web client sends a flood of `Text.handleMutation` transactions and we don't need to replicate any of it.

## Other v3 endpoints observed (not used by this CLI today)

For completeness, these endpoints showed up in the HAR but aren't on the lifecycle path:

- `etClient` — telemetry/event tracking.
- `getBots`, `getJiraAIConnectionStatus`, `getIsCalendarUser`, `getIsMailUser`, `getInferenceTranscriptsUnreadCount`, `listAIConnectors` — workspace feature/integration probes.
- `getPresenceAuthorizationToken` — multiplayer presence.
- `recordPageVisit`, `recordPageExit` — analytics.
- `getExtendedUserProfiles`, `getUserSharedPagesInSpace` — user profile data.
- `getTrustedDomainsForSpace`, `getEmailDomainSettings` — workspace config.
- `getPageVisitors` — who's currently viewing a page.
- `detectPageLanguage` — content language detection.
- `getPublicPageData`, `getPublicSpaceData` — public-share metadata.

None of these are required by lifecycle ops, but if any future feature needs them, request/response shapes can be re-captured from the same HAR.
