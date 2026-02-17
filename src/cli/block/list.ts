import type { Command } from "commander";
import { withBackend } from "../../notion/client.ts";
import { handleAction } from "../../lib/errors.ts";
import { normalizeId } from "../../lib/ids.ts";
import { printJson, printPaginated } from "../../lib/output.ts";
import { blocksToMarkdown, flattenBlock } from "../../notion/markdown.ts";

export function registerList(block: Command): void {
  block
    .command("list")
    .description("List blocks (content) of a page")
    .argument("<page-id>", "Page or block UUID")
    .option("--raw", "Return structured block objects instead of markdown")
    .option("--limit <n>", "Max blocks")
    .option("--cursor <cursor>", "Pagination cursor")
    .action(async (rawPageId: string, opts: Record<string, string | boolean | undefined>) => {
      const pageId = normalizeId(rawPageId);
      await handleAction(async () => {
        if (opts.raw) {
          // Raw mode: paginated block objects
          const result = await withBackend((backend) =>
            backend.listBlocks({
              id: pageId,
              limit: opts.limit ? parseInt(opts.limit as string, 10) : 100,
              cursor: opts.cursor as string | undefined,
            }),
          );

          printPaginated(
            result.items.map(flattenBlock),
            { hasMore: result.hasMore, nextCursor: result.nextCursor },
          );
        } else {
          // Markdown mode: fetch all blocks and convert
          const result = await withBackend(async (backend) => {
            const { blocks, hasMore } = await backend.getAllBlocks(pageId);
            const withChildren = blocks.filter((b) => b.hasChildren);
            const childMap = withChildren.length > 0
              ? await backend.getChildBlocks(withChildren.map((b) => b.id))
              : new Map();

            return {
              pageId,
              content: blocksToMarkdown(blocks, childMap),
              blockCount: blocks.length,
              hasMore,
            };
          });

          printJson(result);
        }
      });
    });
}
