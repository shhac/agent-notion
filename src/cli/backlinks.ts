import type { Command } from "commander";
import { createV3Client } from "../notion/client.ts";
import { handleAction } from "../lib/errors.ts";
import { normalizeId } from "../lib/ids.ts";
import { printJson } from "../lib/output.ts";
import { getBlock } from "../notion/v3/transforms.ts";
import { v3RichTextToPlain } from "../notion/v3/transforms.ts";

export function registerBacklinksCommand(program: Command): void {
  program
    .command("backlinks")
    .description("List pages that link to a given page (v3 desktop session required)")
    .argument("<page-id>", "Page UUID or dashless ID")
    .action(async (rawPageId: string) => {
      await handleAction(async () => {
        const pageId = normalizeId(rawPageId);
        const client = createV3Client();

        const result = await client.getBacklinksForBlock({ blockId: pageId });

        const backlinks = (result.backlinks ?? []).map((bl) => {
          const block = getBlock(result.recordMap, bl.mentioned_from.block_id);
          // Walk up to find the page-level block
          const pageBlock = block?.parent_id
            ? getBlock(result.recordMap, block.parent_id)
            : undefined;

          return {
            blockId: bl.mentioned_from.block_id,
            pageId: pageBlock?.id ?? bl.mentioned_from.block_id,
            pageTitle: pageBlock?.properties?.title
              ? v3RichTextToPlain(pageBlock.properties.title)
              : block?.properties?.title
                ? v3RichTextToPlain(block.properties.title)
                : undefined,
          };
        });

        // Deduplicate by pageId
        const seen = new Set<string>();
        const unique = backlinks.filter((bl) => {
          if (seen.has(bl.pageId)) return false;
          seen.add(bl.pageId);
          return true;
        });

        printJson({ backlinks: unique, total: unique.length });
      });
    });
}
