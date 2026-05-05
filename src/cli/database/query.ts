import { defineCommand, ExactArgs, type Command } from "../../lib/cli.ts";
import { withBackend } from "../../notion/client.ts";
import { handleAction, CliError } from "../../lib/errors.ts";
import { normalizeId } from "../../lib/ids.ts";
import { printPaginated, resolvePageSize } from "../../lib/output.ts";

export function registerQuery(database: Command): void {
  database.addCommand(
    defineCommand({
      use: "query <database-id>",
      short: "Query database rows with optional filters and sorts",
      args: ExactArgs(1),
      options: {
        filter: { type: "string", description: "Notion filter object (JSON)" },
        sort: { type: "string", description: "Notion sort array (JSON)" },
        limit: { type: "string", description: "Max results" },
        cursor: { type: "string", description: "Pagination cursor" },
      },
      action: async ([rawDatabaseId], opts) => {
        const databaseId = normalizeId(rawDatabaseId!);
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
              limit: resolvePageSize({ limit: opts.limit }),
              cursor: opts.cursor,
            }),
          );

          printPaginated(result.items, {
            hasMore: result.hasMore,
            nextCursor: result.nextCursor,
          });
        });
      },
    }),
  );
}
