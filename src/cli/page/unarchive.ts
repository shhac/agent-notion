import { defineCommand, ExactArgs, type Command } from "../../lib/cli.ts";
import { withBackend } from "../../notion/client.ts";
import { handleAction } from "../../lib/errors.ts";
import { normalizeId } from "../../lib/ids.ts";
import { printJson } from "../../lib/output.ts";

export function registerUnarchive(page: Command): void {
  page.addCommand(
    defineCommand({
      use: "unarchive <page-id>",
      short: "Unarchive a page (undo 'page archive' — v3 only)",
      args: ExactArgs(1),
      action: async ([rawPageId]) => {
        const pageId = normalizeId(rawPageId!);
        await handleAction(async () => {
          const result = await withBackend((backend) => backend.unarchivePage(pageId));
          printJson(result);
        });
      },
    }),
  );
}
