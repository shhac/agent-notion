import type { Command } from "commander";
import { withBackend } from "../../notion/client.ts";
import { handleAction } from "../../lib/errors.ts";
import { normalizeId } from "../../lib/ids.ts";
import { printJson } from "../../lib/output.ts";

export function registerAdd(comment: Command): void {
  comment
    .command("add")
    .description("Add a comment to a page")
    .argument("<page-id>", "Page UUID")
    .argument("<body>", "Comment text")
    .action(async (rawPageId: string, body: string) => {
      const pageId = normalizeId(rawPageId);
      await handleAction(async () => {
        const result = await withBackend((backend) =>
          backend.addComment({ pageId, body }),
        );

        printJson(result);
      });
    });
}
