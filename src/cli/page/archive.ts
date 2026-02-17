import type { Command } from "commander";
import { withBackend } from "../../notion/client.ts";
import { handleAction } from "../../lib/errors.ts";
import { printJson } from "../../lib/output.ts";

export function registerArchive(page: Command): void {
  page
    .command("archive")
    .description("Move a page to trash")
    .argument("<page-id>", "Page UUID")
    .action(async (pageId: string) => {
      await handleAction(async () => {
        const result = await withBackend((backend) =>
          backend.archivePage(pageId),
        );

        printJson(result);
      });
    });
}
