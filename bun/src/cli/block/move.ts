import { defineCommand, ExactArgs, type Command } from "../../lib/cli.ts";
import { withBackend } from "../../notion/client.ts";
import { handleAction } from "../../lib/errors.ts";
import { normalizeId } from "../../lib/ids.ts";
import { printJson } from "../../lib/output.ts";

export function registerMove(block: Command): void {
  block.addCommand(
    defineCommand({
      use: "move <block-id>",
      short: "Move a block to a new position or parent (v3 only)",
      args: ExactArgs(1),
      options: {
        parent: {
          type: "string",
          description: "New parent block (for moving into a container)",
        },
        after: {
          type: "string",
          description: "Place after this block (omit for first position)",
        },
      },
      action: async ([rawBlockId], opts) => {
        const blockId = normalizeId(rawBlockId!);
        await handleAction(async () => {
          const parentId = opts.parent ? normalizeId(opts.parent) : undefined;
          const afterId = opts.after ? normalizeId(opts.after) : undefined;

          const result = await withBackend((backend) =>
            backend.moveBlock({ id: blockId, parentId, afterId }),
          );
          printJson(result);
        });
      },
    }),
  );
}
