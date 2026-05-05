import { defineCommand, ExactArgs, type Command } from "../../lib/cli.ts";
import { withBackend } from "../../notion/client.ts";
import { handleAction } from "../../lib/errors.ts";
import { normalizeId } from "../../lib/ids.ts";
import { printJson } from "../../lib/output.ts";
import { blocksToMarkdown, flattenBlock } from "../../notion/markdown.ts";

export function registerGet(page: Command): void {
  page.addCommand(
    defineCommand({
      use: "get <page-id>",
      short: "Get page properties and optionally content",
      args: ExactArgs(1),
      options: {
        content: { type: "bool", description: "Include page content as markdown" },
        rawContent: {
          type: "bool",
          description: "Include content as structured block objects",
        },
      },
      action: async ([rawPageId], opts) => {
        const pageId = normalizeId(rawPageId!);
        await handleAction(async () => {
          const result = await withBackend(async (backend) => {
            const page = await backend.getPage(pageId);
            const output: Record<string, unknown> = { ...page };

            if (opts.content || opts.rawContent) {
              const { blocks, hasMore } = await backend.getAllBlocks(pageId);

              if (opts.rawContent) {
                output.blocks = blocks.map(flattenBlock);
                output.blockCount = blocks.length;
                if (hasMore) output.contentTruncated = true;
              } else {
                const withChildren = blocks.filter((b) => b.hasChildren);
                const childMap =
                  withChildren.length > 0
                    ? await backend.getChildBlocks(withChildren.map((b) => b.id))
                    : new Map();
                output.content = blocksToMarkdown(blocks, childMap);
                output.blockCount = blocks.length;
                if (hasMore) output.contentTruncated = true;
              }
            }

            return output;
          });

          printJson(result);
        });
      },
    }),
  );
}
