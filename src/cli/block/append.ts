import type { Command } from "commander";
import { withBackend } from "../../notion/client.ts";
import { handleAction, CliError } from "../../lib/errors.ts";
import { printJson } from "../../lib/output.ts";
import { markdownToBlocks } from "../../notion/markdown.ts";

export function registerAppend(block: Command): void {
  block
    .command("append")
    .description("Append blocks to a page")
    .argument("<page-id>", "Page or block UUID")
    .option("--content <markdown>", "Content as markdown")
    .option("--blocks <json>", "Content as Notion block objects (JSON array)")
    .action(async (pageId: string, opts: Record<string, string | undefined>) => {
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

        const result = await withBackend((backend) =>
          backend.appendBlocks({ id: pageId, blocks: children }),
        );

        printJson({ pageId, ...result });
      });
    });
}
