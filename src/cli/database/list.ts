import type { Command } from "commander";
import { withBackend } from "../../notion/client.ts";
import { handleAction } from "../../lib/errors.ts";
import { printPaginated, resolvePageSize } from "../../lib/output.ts";

export function registerList(database: Command): void {
  database
    .command("list")
    .description("List all databases the integration can access")
    .option("--limit <n>", "Max results")
    .option("--cursor <cursor>", "Pagination cursor")
    .action(async (opts: Record<string, string | undefined>) => {
      await handleAction(async () => {
        const result = await withBackend((backend) =>
          backend.listDatabases({
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
