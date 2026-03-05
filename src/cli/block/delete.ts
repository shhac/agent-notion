import type { Command } from "commander";
import { withBackend } from "../../notion/client.ts";
import { handleAction } from "../../lib/errors.ts";
import { normalizeId } from "../../lib/ids.ts";
import { printJson } from "../../lib/output.ts";

export function registerDelete(block: Command): void {
  block
    .command("delete")
    .description("Delete a block")
    .argument("<block-id>", "Block UUID to delete")
    .action(async (rawBlockId: string) => {
      const blockId = normalizeId(rawBlockId);
      await handleAction(async () => {
        const result = await withBackend((backend) =>
          backend.deleteBlock(blockId),
        );
        printJson(result);
      });
    });
}
