import type { Command } from "commander";

const USAGE_TEXT = `agent-notion block — Read and write page content (blocks)

LIST (read content):
  block list <page-id>                                  Content as markdown (default)
  block list <page-id> --raw                            Content as structured block objects
  block list <page-id> --raw --limit 10 --cursor <c>    Paginate raw blocks

  Markdown output: { pageId, content, blockCount, hasMore }
  Raw output: { items: [{ id, type, content, hasChildren }], pagination? }

APPEND (write content):
  block append <page-id> --content <markdown>   Append markdown (converted to blocks)
  block append <page-id> --blocks <json>        Append raw Notion block objects

  Output: { pageId, blocksAppended }
  Markdown conversion supports: headings, lists, todos, code blocks, quotes, dividers.

LIMITS:
  Max 1000 blocks per request (list or append).
  500KB payload limit on append.
  2000 character limit per rich text object.
  Only appends — cannot insert at specific positions or update existing blocks.

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
