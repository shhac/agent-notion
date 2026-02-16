import type { Command } from "commander";
import { withAutoRefresh } from "../../notion/client.ts";
import { handleAction } from "../../lib/errors.ts";
import { printPaginated, resolvePageSize } from "../../lib/output.ts";

export function registerList(comment: Command): void {
  comment
    .command("list")
    .description("List comments on a page or block")
    .argument("<page-id>", "Page or block UUID")
    .option("--limit <n>", "Max results")
    .option("--cursor <cursor>", "Pagination cursor")
    .action(async (pageId: string, opts: Record<string, string | undefined>) => {
      await handleAction(async () => {
        const results = await withAutoRefresh((client) =>
          client.comments.list({
            block_id: pageId,
            page_size: resolvePageSize(opts),
            start_cursor: opts.cursor,
          } as Parameters<typeof client.comments.list>[0]),
        );

        const items = results.results.map((c: Record<string, unknown>) => {
          const comment = c as {
            id: string;
            rich_text?: Array<{ plain_text: string }>;
            created_by?: { id: string; name?: string };
            created_time?: string;
          };

          return {
            id: comment.id,
            body: comment.rich_text?.map((t) => t.plain_text).join("") ?? "",
            author: comment.created_by
              ? { id: comment.created_by.id, name: comment.created_by.name }
              : null,
            createdAt: comment.created_time,
          };
        });

        printPaginated(items, {
          hasMore: results.has_more,
          nextCursor: results.next_cursor,
        });
      });
    });
}
