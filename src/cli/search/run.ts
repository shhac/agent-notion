import type { Command } from "commander";
import { withBackend } from "../../notion/client.ts";
import { handleAction } from "../../lib/errors.ts";
import { printPaginated, resolvePageSize } from "../../lib/output.ts";

export function registerRun(search: Command): void {
  search
    .argument("<query>", "Search text (matched against titles)")
    .option("--filter <type>", "Filter by type: page | database")
    .option("--limit <n>", "Max results")
    .option("--cursor <cursor>", "Pagination cursor")
    .action(async (query: string, opts: Record<string, string | boolean | undefined>) => {
      await handleAction(async () => {
        const result = await withBackend((backend) =>
          backend.search({
            query,
            filter: opts.filter as "page" | "database" | undefined,
            limit: resolvePageSize(opts as Record<string, string | undefined>),
            cursor: opts.cursor as string | undefined,
          }),
        );

        printPaginated(result.items, {
          hasMore: result.hasMore,
          nextCursor: result.nextCursor,
        });
      });
    });
}
