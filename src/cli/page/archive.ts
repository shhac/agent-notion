import { defineCommand, ExactArgs, type Command } from "../../lib/cli.ts";
import { withBackend } from "../../notion/client.ts";
import { handleAction } from "../../lib/errors.ts";
import { normalizeId } from "../../lib/ids.ts";
import { printJson } from "../../lib/output.ts";

export function registerArchive(page: Command): void {
  page.addCommand(
    defineCommand({
      use: "archive <page-id>",
      short: "Move a page to trash",
      args: ExactArgs(1),
      action: async ([rawPageId]) => {
        const pageId = normalizeId(rawPageId!);
        await handleAction(async () => {
          const result = await withBackend((backend) => backend.archivePage(pageId));
          printJson(result);
        });
      },
    }),
  );
}
