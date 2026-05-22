import { defineCommand, type Command } from "../../lib/cli.ts";

const USAGE_TEXT = `agent-notion page — Page operations

GET:
  page get <page-id>                         Properties only
  page get <page-id> --content               Properties + markdown content
  page get <page-id> --raw-content           Properties + block objects

CREATE:
  page create --parent <id> --title <title> [--properties <json>] [--icon <emoji>]

  Database parent example:
    page create --parent <db-id> --title "New Task" --properties '{"Status":"Not started","Priority":"High"}'

UPDATE:
  page update <page-id> [--title <title>] [--properties <json>] [--icon <emoji>]
  Page content cannot be updated here — use "block append <id> --content <md>".

TRASH / RESTORE / ARCHIVE:
  page trash     <page-id>                   Move to Trash (alive=false, recoverable)
  page restore   <page-id>                   Restore from Trash
  page archive   <page-id>                   Archive: hide from search, page stays alive (v3)
  page unarchive <page-id>                   Undo archive (v3)

  Archive and Trash are independent states. v3 commands need: auth import-desktop.

BACKLINKS (v3):
  page backlinks <page-id>                   Pages that link to a given page
  Output: { "backlinks": [{ blockId, pageId, pageTitle }], "total": <n> }

HISTORY (v3):
  page history <page-id> [--limit <n>]       Version history (snapshots) for a page
  Output: { "snapshots": [{ id, version, lastVersion, timestamp, authors }], "total": <n> }

CONTENT FORMAT: When using --content, blocks are converted to markdown:
  headings → #/##/### | lists → -/1. | todos → - [ ]/- [x]
  code → fenced blocks | quotes → > | callouts → > icon text
  images → ![caption](url) | dividers → ---

LIMITS: Max 1000 blocks per page fetch. contentTruncated=true if exceeded.
FILE URLS: Image/file URLs expire after ~1 hour. Re-fetch for fresh URLs.
`;

export function registerUsage(page: Command): void {
  page.addCommand(
    defineCommand({
      use: "usage",
      short: "Print detailed page documentation (LLM-optimized)",
      action: () => {
        console.log(USAGE_TEXT.trim());
      },
    }),
  );
}
