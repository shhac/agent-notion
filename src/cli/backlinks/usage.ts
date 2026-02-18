import type { Command } from "commander";

const USAGE_TEXT = `agent-notion backlinks — Pages that link to a given page

USAGE:
  backlinks <page-id>

  Requires a v3 desktop session (auth import-desktop).

ARGUMENTS:
  <page-id>    Page UUID or dashless ID

OUTPUT:
  { "backlinks": [{ blockId, pageId, pageTitle }], "total": <n> }

  blockId: The specific block containing the link
  pageId: The page containing that block (deduplicated)
  pageTitle: Resolved page title from workspace data

NOTES:
  Results are deduplicated by pageId — each linking page appears at most once.
  Backlinks include @-mentions and inline page references.

EXAMPLES:
  backlinks abc123                     Find pages linking to a page
  backlinks 12345678-abcd-1234-...     Full UUID format also accepted
`;

export function registerUsage(backlinks: Command): void {
  backlinks
    .command("usage")
    .description("Print detailed backlinks documentation (LLM-optimized)")
    .action(() => {
      console.log(USAGE_TEXT.trim());
    });
}
