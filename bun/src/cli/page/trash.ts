import { defineCommand, ExactArgs, type Command } from "../../lib/cli.ts";
import { withBackend } from "../../notion/client.ts";
import { handleAction } from "../../lib/errors.ts";
import { normalizeId } from "../../lib/ids.ts";
import { printJson } from "../../lib/output.ts";

export function registerTrash(page: Command): void {
  page.addCommand(
    defineCommand({
      use: "trash <page-id>",
      short: "Move a page to Trash (recoverable with 'page restore')",
      args: ExactArgs(1),
      action: async ([rawPageId]) => {
        const pageId = normalizeId(rawPageId!);
        await handleAction(async () => {
          const result = await withBackend((backend) => backend.trashPage(pageId));
          printJson(result);
        });
      },
    }),
  );
}
