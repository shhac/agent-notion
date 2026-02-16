import type { Command } from "commander";
import { withAutoRefresh } from "../../notion/client.ts";
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
        const results = await withAutoRefresh((client) =>
          client.search({
            filter: { property: "object", value: "database" },
            page_size: resolvePageSize(opts),
            start_cursor: opts.cursor,
          } as Parameters<typeof client.search>[0]),
        );

        const items = results.results.map((item: Record<string, unknown>) => {
          const db = item as {
            id: string;
            url: string;
            title?: Array<{ plain_text: string }>;
            parent?: Record<string, unknown>;
            properties?: Record<string, unknown>;
            last_edited_time?: string;
          };

          return {
            id: db.id,
            title: db.title?.map((t) => t.plain_text).join("") ?? "",
            url: db.url,
            parent: formatParent(db.parent),
            propertyCount: db.properties ? Object.keys(db.properties).length : 0,
            lastEditedAt: db.last_edited_time,
          };
        });

        printPaginated(items, {
          hasMore: results.has_more,
          nextCursor: results.next_cursor,
        });
      });
    });
}

function formatParent(parent: Record<string, unknown> | undefined): Record<string, unknown> | undefined {
  if (!parent) return undefined;
  if (parent.type === "page_id") return { type: "page", id: parent.page_id as string };
  if (parent.type === "workspace") return { type: "workspace" };
  if (parent.type === "block_id") return { type: "block", id: parent.block_id as string };
  return undefined;
}
