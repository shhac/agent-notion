import type { Command } from "commander";
import type { Client } from "@notionhq/client";
import { withAutoRefresh } from "../../notion/client.ts";
import { handleAction } from "../../lib/errors.ts";
import { printJson, printPaginated } from "../../lib/output.ts";
import { blocksToMarkdown, flattenBlock } from "../../notion/blocks.ts";

export function registerList(block: Command): void {
  block
    .command("list")
    .description("List blocks (content) of a page")
    .argument("<page-id>", "Page or block UUID")
    .option("--raw", "Return structured block objects instead of markdown")
    .option("--limit <n>", "Max blocks")
    .option("--cursor <cursor>", "Pagination cursor")
    .action(async (pageId: string, opts: Record<string, string | boolean | undefined>) => {
      await handleAction(async () => {
        if (opts.raw) {
          // Raw mode: paginated block objects
          await withAutoRefresh(async (client) => {
            const response = await client.blocks.children.list({
              block_id: pageId,
              page_size: opts.limit ? parseInt(opts.limit as string, 10) : 100,
              start_cursor: opts.cursor as string | undefined,
            });

            const items = response.results.map((b) =>
              flattenBlock(b as Parameters<typeof flattenBlock>[0]),
            );

            printPaginated(items, {
              hasMore: response.has_more,
              nextCursor: response.next_cursor,
            });
          });
        } else {
          // Markdown mode: fetch all blocks and convert
          await withAutoRefresh(async (client) => {
            const { blocks, hasMore } = await fetchAllBlocks(client, pageId);
            const childMap = await fetchChildBlocks(client, blocks);

            const content = blocksToMarkdown(
              blocks as Parameters<typeof blocksToMarkdown>[0],
              childMap as Parameters<typeof blocksToMarkdown>[1],
            );

            printJson({
              pageId,
              content,
              blockCount: blocks.length,
              hasMore,
            });
          });
        }
      });
    });
}

async function fetchAllBlocks(
  client: Client,
  blockId: string,
): Promise<{ blocks: Record<string, unknown>[]; hasMore: boolean }> {
  const blocks: Record<string, unknown>[] = [];
  let cursor: string | undefined;
  let hasMore = false;

  while (blocks.length < 1000) {
    const response = await client.blocks.children.list({
      block_id: blockId,
      page_size: 100,
      start_cursor: cursor,
    });

    blocks.push(...(response.results as Record<string, unknown>[]));

    if (!response.has_more) break;
    if (blocks.length >= 1000) {
      hasMore = true;
      break;
    }
    cursor = response.next_cursor ?? undefined;
  }

  return { blocks, hasMore };
}

async function fetchChildBlocks(
  client: Client,
  blocks: Record<string, unknown>[],
): Promise<Map<string, Record<string, unknown>[]>> {
  const childMap = new Map<string, Record<string, unknown>[]>();
  const withChildren = blocks.filter((b) => b.has_children);

  for (let i = 0; i < withChildren.length; i += 5) {
    const batch = withChildren.slice(i, i + 5);
    await Promise.all(
      batch.map(async (block) => {
        const { blocks: children } = await fetchAllBlocks(client, block.id as string);
        childMap.set(block.id as string, children);
      }),
    );
  }

  return childMap;
}
