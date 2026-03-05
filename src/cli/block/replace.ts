import type { Command } from "commander";
import { withBackend } from "../../notion/client.ts";
import { handleAction, CliError } from "../../lib/errors.ts";
import { normalizeId } from "../../lib/ids.ts";
import { printJson } from "../../lib/output.ts";
import { markdownToBlocks } from "../../notion/markdown.ts";

export function registerReplace(block: Command): void {
  block
    .command("replace")
    .description("Replace all blocks on a page (deletes existing, appends new)")
    .argument("<page-id>", "Page or block UUID")
    .option("--content <markdown>", "New content as markdown")
    .option("--blocks <json>", "New content as Notion block objects (JSON array)")
    .action(async (rawPageId: string, opts: Record<string, string | undefined>) => {
      const pageId = normalizeId(rawPageId);
      await handleAction(async () => {
        if (!opts.content && !opts.blocks) {
          throw new CliError("Provide --content (markdown) or --blocks (JSON array).");
        }

        let children: unknown[];

        if (opts.blocks) {
          try {
            children = JSON.parse(opts.blocks);
            if (!Array.isArray(children)) {
              throw new CliError("--blocks must be a JSON array of block objects.");
            }
          } catch (e) {
            if (e instanceof CliError) throw e;
            throw new CliError(
              "Invalid --blocks JSON. Expected an array of Notion block objects.",
            );
          }
        } else {
          children = markdownToBlocks(opts.content!);
        }

        const result = await withBackend(async (backend) => {
          // 1. Fetch existing blocks
          const { blocks: existing } = await backend.getAllBlocks(pageId);

          // 2. Delete all existing blocks
          for (const block of existing) {
            await backend.deleteBlock(block.id);
          }

          // 3. Append new blocks
          let blocksAdded = 0;
          if (children.length > 0) {
            const appended = await backend.appendBlocks({ id: pageId, blocks: children });
            blocksAdded = appended.blocksAdded;
          }

          return {
            pageId,
            blocksDeleted: existing.length,
            blocksAdded,
          };
        });

        printJson(result);
      });
    });
}
