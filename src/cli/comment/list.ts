import type { Command } from "commander";
import { withBackend } from "../../notion/client.ts";
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
        const result = await withBackend((backend) =>
          backend.listComments({
            pageId,
            limit: resolvePageSize(opts),
            cursor: opts.cursor,
          }),
        );

        printPaginated(result.items, {
          hasMore: result.hasMore,
          nextCursor: result.nextCursor,
        });
      });
    });
}
