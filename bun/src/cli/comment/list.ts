import { defineCommand, ExactArgs, type Command } from "../../lib/cli.ts";
import { withBackend } from "../../notion/client.ts";
import { handleAction } from "../../lib/errors.ts";
import { normalizeId } from "../../lib/ids.ts";
import { printPaginated, resolvePageSize } from "../../lib/output.ts";

export function registerList(comment: Command): void {
  comment.addCommand(
    defineCommand({
      use: "list <page-id>",
      short: "List comments on a page or block",
      args: ExactArgs(1),
      options: {
        limit: { type: "string", description: "Max results" },
        cursor: { type: "string", description: "Pagination cursor" },
      },
      action: async ([rawPageId], opts) => {
        const pageId = normalizeId(rawPageId!);
        await handleAction(async () => {
          const result = await withBackend((backend) =>
            backend.listComments({
              pageId,
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
