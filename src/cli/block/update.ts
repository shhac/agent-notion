import { defineCommand, ExactArgs, type Command } from "../../lib/cli.ts";
import { withBackend } from "../../notion/client.ts";
import { handleAction, CliError } from "../../lib/errors.ts";
import { normalizeId } from "../../lib/ids.ts";
import { printJson } from "../../lib/output.ts";

export function registerUpdate(block: Command): void {
  block.addCommand(
    defineCommand({
      use: "update <block-id>",
      short: "Update a block's content",
      args: ExactArgs(1),
      options: {
        content: { type: "string", description: "New text content for the block" },
      },
      action: async ([rawBlockId], opts) => {
        const blockId = normalizeId(rawBlockId!);
        await handleAction(async () => {
          if (!opts.content) {
            throw new CliError("Provide --content with the new text.");
          }

          const result = await withBackend((backend) =>
            backend.updateBlock({ id: blockId, content: opts.content }),
          );
          printJson(result);
        });
      },
    }),
  );
}
