import type { Command } from "commander";

const USAGE_TEXT = `agent-notion block — Read and write page content (blocks)

LIST (read content):
  block list <page-id>                                  Content as markdown (default)
  block list <page-id> --raw                            Content as structured block objects
  block list <page-id> --raw --limit 10 --cursor <c>    Paginate raw blocks

  Markdown output: { pageId, content, blockCount, hasMore }
  Raw output: { items: [{ id, type, content, hasChildren }], pagination? }

APPEND (add content):
  block append <page-id> --content <markdown>   Append markdown (converted to blocks)
  block append <page-id> --blocks <json>        Append raw Notion block objects

  Output: { pageId, blocksAdded }

UPDATE (edit existing block):
  block update <block-id> --content <text>      Replace the text content of a block

  Output: { id, lastEditedAt }
  Use "block list --raw" to get block IDs first.

DELETE (remove a block):
  block delete <block-id>                       Delete a single block

  Output: { id, deleted: true }

MOVE (reorder / reparent):                                               ◆ v3
  block move <block-id> --after <block-id>      Move after a specific block
  block move <block-id>                         Move to first position in parent
  block move <block-id> --parent <block-id>     Move into a container block (first child)
  block move <id> --parent <id> --after <id>    Move into container after a sibling

  Output: { id, parentId, afterId? }

REPLACE (replace all page content):
  block replace <page-id> --content <markdown>  Delete all blocks, then append new content
  block replace <page-id> --blocks <json>       Delete all blocks, then append raw blocks

  Output: { pageId, blocksDeleted, blocksAdded }

LIMITS:
  Max 1000 blocks per request (list or append).
  500KB payload limit on append.
  2000 character limit per rich text object.
  Cannot insert at specific positions — use replace for full rewrites.

NESTED BLOCKS: Blocks with children (toggles, columns) are recursively fetched in
  markdown mode. In raw mode, check hasChildren and re-fetch if needed.
`;

export function registerUsage(block: Command): void {
  block
    .command("usage")
    .description("Print detailed block documentation (LLM-optimized)")
    .action(() => {
      console.log(USAGE_TEXT.trim());
    });
}
