import type { Command } from "commander";
import { withBackend } from "../../notion/client.ts";
import { handleAction } from "../../lib/errors.ts";
import { normalizeId } from "../../lib/ids.ts";
import { printJson } from "../../lib/output.ts";

export function registerArchive(page: Command): void {
  page
    .command("archive")
    .description("Move a page to trash")
    .argument("<page-id>", "Page UUID")
    .action(async (rawPageId: string) => {
      const pageId = normalizeId(rawPageId);
      await handleAction(async () => {
        const result = await withBackend((backend) =>
          backend.archivePage(pageId),
        );

        printJson(result);
      });
    });
}
