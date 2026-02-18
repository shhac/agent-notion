import type { Command } from "commander";

const USAGE_TEXT = `agent-notion page — Page operations (get, create, update, archive, backlinks, history)

GET:
  page get <page-id>                         Page properties only
  page get <page-id> --content               Properties + content as markdown
  page get <page-id> --raw-content           Properties + content as block objects

CREATE:
  page create --parent <id> --title <title>  Create page
    [--properties <json>]                    Property values (for database parents)
    [--icon <emoji>]                         Page icon

  Database parent example:
    page create --parent <db-id> --title "New Task" --properties '{"Status":"Not started","Priority":"High"}'

  Page parent example:
    page create --parent <page-id> --title "Sub-page"

UPDATE:
  page update <page-id> [--title <title>] [--properties <json>] [--icon <emoji>]

  Note: Page content (blocks) cannot be updated via page update.
  Use "block append <id> --content <md>" to add content.

ARCHIVE:
  page archive <page-id>                     Move to trash (no permanent delete via API)

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
  page
    .command("usage")
    .description("Print detailed page documentation (LLM-optimized)")
    .action(() => {
      console.log(USAGE_TEXT.trim());
    });
}
