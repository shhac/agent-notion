import type { Command } from "commander";
import { withBackend } from "../../notion/client.ts";
import { handleAction, CliError } from "../../lib/errors.ts";
import { normalizeId } from "../../lib/ids.ts";
import { printPaginated, resolvePageSize } from "../../lib/output.ts";

export function registerQuery(database: Command): void {
  database
    .command("query")
    .description("Query database rows with optional filters and sorts")
    .argument("<database-id>", "Database UUID")
    .option("--filter <json>", "Notion filter object (JSON)")
    .option("--sort <json>", "Notion sort array (JSON)")
    .option("--limit <n>", "Max results")
    .option("--cursor <cursor>", "Pagination cursor")
    .action(async (rawDatabaseId: string, opts: Record<string, string | undefined>) => {
      const databaseId = normalizeId(rawDatabaseId);
      await handleAction(async () => {
        let filter: unknown;
        if (opts.filter) {
          try {
            filter = JSON.parse(opts.filter);
          } catch {
            throw new CliError(
              "Invalid --filter JSON. Expected a Notion filter object. Run 'agent-notion database schema <id>' to see available properties and types.",
            );
          }
        }

        let sort: unknown;
        if (opts.sort) {
          try {
            sort = JSON.parse(opts.sort);
            if (!Array.isArray(sort)) sort = [sort];
          } catch {
            throw new CliError(
              "Invalid --sort JSON. Expected a sort object or array. Example: '[{\"property\":\"Name\",\"direction\":\"ascending\"}]'",
            );
          }
        }

        const result = await withBackend((backend) =>
          backend.queryDatabase({
            id: databaseId,
            filter,
            sort,
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
