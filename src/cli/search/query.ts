import { defineCommand, ExactArgs, type Command } from "../../lib/cli.ts";
import { withBackend } from "../../notion/client.ts";
import { handleAction } from "../../lib/errors.ts";
import { printPaginated, resolvePageSize } from "../../lib/output.ts";

export function registerQuery(search: Command): void {
  search.addCommand(
    defineCommand({
      use: "query <query>",
      short: "Search Notion by title (pages and databases)",
      args: ExactArgs(1),
      options: {
        filter: { type: "string", description: "Filter by type: page | database" },
        limit: { type: "string", description: "Max results" },
        cursor: { type: "string", description: "Pagination cursor" },
      },
      action: async ([query], opts) => {
        await handleAction(async () => {
          const result = await withBackend((backend) =>
            backend.search({
              query: query!,
              filter: opts.filter as "page" | "database" | undefined,
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
