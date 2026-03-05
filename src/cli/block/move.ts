import type { Command } from "commander";
import { withBackend } from "../../notion/client.ts";
import { handleAction } from "../../lib/errors.ts";
import { normalizeId } from "../../lib/ids.ts";
import { printJson } from "../../lib/output.ts";

export function registerMove(block: Command): void {
  block
    .command("move")
    .description("Move a block to a new position or parent (v3 only)")
    .argument("<block-id>", "Block UUID to move")
    .option("--parent <block-id>", "New parent block (for moving into a container)")
    .option("--after <block-id>", "Place after this block (omit for first position)")
    .action(async (rawBlockId: string, opts: Record<string, string | undefined>) => {
      const blockId = normalizeId(rawBlockId);
      await handleAction(async () => {
        const parentId = opts.parent ? normalizeId(opts.parent) : undefined;
        const afterId = opts.after ? normalizeId(opts.after) : undefined;

        const result = await withBackend((backend) =>
          backend.moveBlock({ id: blockId, parentId, afterId }),
        );
        printJson(result);
      });
    });
}
