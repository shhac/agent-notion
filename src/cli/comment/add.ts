import type { Command } from "commander";
import { withAutoRefresh } from "../../notion/client.ts";
import { handleAction } from "../../lib/errors.ts";
import { printJson } from "../../lib/output.ts";

export function registerAdd(comment: Command): void {
  comment
    .command("add")
    .description("Add a comment to a page")
    .argument("<page-id>", "Page UUID")
    .argument("<body>", "Comment text")
    .action(async (pageId: string, body: string) => {
      await handleAction(async () => {
        const result = await withAutoRefresh((client) =>
          client.comments.create({
            parent: { page_id: pageId },
            rich_text: [{ type: "text", text: { content: body } }],
          } as Parameters<typeof client.comments.create>[0]),
        );

        const c = result as Record<string, unknown>;
        printJson({
          id: c.id,
          body,
          createdAt: c.created_time,
        });
      });
    });
}
