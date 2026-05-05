import { defineCommand, ExactArgs, type Command } from "../../lib/cli.ts";
import { createV3Client } from "../../notion/client.ts";
import { handleAction } from "../../lib/errors.ts";
import { normalizeId } from "../../lib/ids.ts";
import { printJson } from "../../lib/output.ts";
import { getBlock, v3RichTextToPlain } from "../../notion/v3/transforms.ts";

export function registerBacklinks(page: Command): void {
  page.addCommand(
    defineCommand({
      use: "backlinks <page-id>",
      short:
        "List pages that link to a given page (v3 desktop session required)",
      args: ExactArgs(1),
      action: async ([rawPageId]) => {
        await handleAction(async () => {
          const pageId = normalizeId(rawPageId!);
          const client = createV3Client();

          const result = await client.getBacklinksForBlock({ blockId: pageId });

          const backlinks = (result.backlinks ?? []).map((bl) => {
            const block = getBlock(result.recordMap, bl.mentioned_from.block_id);
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

          const seen = new Set<string>();
          const unique = backlinks.filter((bl) => {
            if (seen.has(bl.pageId)) return false;
            seen.add(bl.pageId);
            return true;
          });

          printJson({ backlinks: unique, total: unique.length });
        });
      },
    }),
  );
}
