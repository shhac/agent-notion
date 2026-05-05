import { defineCommand, ExactArgs, type Command } from "../../lib/cli.ts";
import { withBackend } from "../../notion/client.ts";
import { handleAction } from "../../lib/errors.ts";
import { normalizeId } from "../../lib/ids.ts";
import { printJson } from "../../lib/output.ts";

export function registerDelete(block: Command): void {
  block.addCommand(
    defineCommand({
      use: "delete <block-id>",
      short: "Delete a block",
      args: ExactArgs(1),
      action: async ([rawBlockId]) => {
        const blockId = normalizeId(rawBlockId!);
        await handleAction(async () => {
          const result = await withBackend((backend) => backend.deleteBlock(blockId));
          printJson(result);
        });
      },
    }),
  );
}
