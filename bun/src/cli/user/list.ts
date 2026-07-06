import { defineCommand, type Command } from "../../lib/cli.ts";
import { withBackend } from "../../notion/client.ts";
import { handleAction } from "../../lib/errors.ts";
import { printPaginated, resolvePageSize } from "../../lib/output.ts";

export function registerList(user: Command): void {
  user.addCommand(
    defineCommand({
      use: "list",
      short: "List all users in the workspace",
      options: {
        limit: { type: "string", description: "Max results" },
        cursor: { type: "string", description: "Pagination cursor" },
      },
      action: async (_args, opts) => {
        await handleAction(async () => {
          const result = await withBackend((backend) =>
            backend.listUsers({
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
