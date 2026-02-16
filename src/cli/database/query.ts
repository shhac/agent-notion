import type { Command } from "commander";
import { withAutoRefresh } from "../../notion/client.ts";
import { handleAction, CliError } from "../../lib/errors.ts";
import { printPaginated, resolvePageSize } from "../../lib/output.ts";
import { flattenProperties } from "../../notion/properties.ts";

export function registerQuery(database: Command): void {
  database
    .command("query")
    .description("Query database rows with optional filters and sorts")
    .argument("<database-id>", "Database UUID")
    .option("--filter <json>", "Notion filter object (JSON)")
    .option("--sort <json>", "Notion sort array (JSON)")
    .option("--limit <n>", "Max results")
    .option("--cursor <cursor>", "Pagination cursor")
    .action(async (databaseId: string, opts: Record<string, string | undefined>) => {
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

        let sorts: unknown;
        if (opts.sort) {
          try {
            sorts = JSON.parse(opts.sort);
            if (!Array.isArray(sorts)) sorts = [sorts];
          } catch {
            throw new CliError(
              "Invalid --sort JSON. Expected a sort object or array. Example: '[{\"property\":\"Name\",\"direction\":\"ascending\"}]'",
            );
          }
        }

        const queryParams: Record<string, unknown> = {
          database_id: databaseId,
          page_size: resolvePageSize(opts),
        };
        if (filter) queryParams.filter = filter;
        if (sorts) queryParams.sorts = sorts;
        if (opts.cursor) queryParams.start_cursor = opts.cursor;

        const results = await withAutoRefresh((client) =>
          client.databases.query(queryParams as Parameters<typeof client.databases.query>[0]),
        );

        const items = results.results.map((page: Record<string, unknown>) => {
          const p = page as {
            id: string;
            url: string;
            properties: Record<string, Record<string, unknown>>;
            created_time?: string;
            last_edited_time?: string;
          };

          return {
            id: p.id,
            url: p.url,
            properties: flattenProperties(
              p.properties as Parameters<typeof flattenProperties>[0],
            ),
            createdAt: p.created_time,
            lastEditedAt: p.last_edited_time,
          };
        });

        printPaginated(items, {
          hasMore: results.has_more,
          nextCursor: results.next_cursor,
        });
      });
    });
}
