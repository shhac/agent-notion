import { defineCommand, ExactArgs, type Command } from "../../lib/cli.ts";
import { withBackend } from "../../notion/client.ts";
import { handleAction } from "../../lib/errors.ts";
import { normalizeId } from "../../lib/ids.ts";
import { printJson, printPaginated } from "../../lib/output.ts";
import { blocksToMarkdown, flattenBlock } from "../../notion/markdown.ts";

export function registerList(block: Command): void {
  block.addCommand(
    defineCommand({
      use: "list <page-id>",
      short: "List blocks (content) of a page",
      args: ExactArgs(1),
      options: {
        raw: {
          type: "bool",
          description: "Return structured block objects instead of markdown",
        },
        limit: { type: "string", description: "Max blocks" },
        cursor: { type: "string", description: "Pagination cursor" },
      },
      action: async ([rawPageId], opts) => {
        const pageId = normalizeId(rawPageId!);
        await handleAction(async () => {
          if (opts.raw) {
            const result = await withBackend((backend) =>
              backend.listBlocks({
                id: pageId,
                limit: opts.limit ? parseInt(opts.limit, 10) : 100,
                cursor: opts.cursor,
              }),
            );

            printPaginated(result.items.map(flattenBlock), {
              hasMore: result.hasMore,
              nextCursor: result.nextCursor,
            });
          } else {
            const result = await withBackend(async (backend) => {
              const { blocks, hasMore } = await backend.getAllBlocks(pageId);
              const withChildren = blocks.filter((b) => b.hasChildren);
              const childMap =
                withChildren.length > 0
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
      },
    }),
  );
}
