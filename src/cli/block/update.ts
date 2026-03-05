import type { Command } from "commander";
import { withBackend } from "../../notion/client.ts";
import { handleAction, CliError } from "../../lib/errors.ts";
import { normalizeId } from "../../lib/ids.ts";
import { printJson } from "../../lib/output.ts";

export function registerUpdate(block: Command): void {
  block
    .command("update")
    .description("Update a block's content")
    .argument("<block-id>", "Block UUID to update")
    .option("--content <text>", "New text content for the block")
    .action(async (rawBlockId: string, opts: Record<string, string | undefined>) => {
      const blockId = normalizeId(rawBlockId);
      await handleAction(async () => {
        if (!opts.content) {
          throw new CliError("Provide --content with the new text.");
        }

        const result = await withBackend((backend) =>
          backend.updateBlock({ id: blockId, content: opts.content }),
        );
        printJson(result);
      });
    });
}
