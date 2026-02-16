import type { Command } from "commander";
import { withAutoRefresh } from "../../notion/client.ts";
import { handleAction } from "../../lib/errors.ts";
import { printPaginated, resolvePageSize } from "../../lib/output.ts";

export function registerList(user: Command): void {
  user
    .command("list")
    .description("List all users in the workspace")
    .option("--limit <n>", "Max results")
    .option("--cursor <cursor>", "Pagination cursor")
    .action(async (opts: Record<string, string | undefined>) => {
      await handleAction(async () => {
        const results = await withAutoRefresh((client) =>
          client.users.list({
            page_size: resolvePageSize(opts),
            start_cursor: opts.cursor,
          } as Parameters<typeof client.users.list>[0]),
        );

        const items = results.results.map((u: Record<string, unknown>) => {
          const user = u as {
            id: string;
            name?: string;
            type?: string;
            person?: { email?: string };
            avatar_url?: string;
          };

          return {
            id: user.id,
            name: user.name,
            type: user.type,
            email: user.person?.email,
            avatarUrl: user.avatar_url,
          };
        });

        printPaginated(items, {
          hasMore: results.has_more,
          nextCursor: results.next_cursor,
        });
      });
    });
}
