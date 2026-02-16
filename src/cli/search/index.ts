import type { Command } from "commander";
import { withAutoRefresh } from "../../notion/client.ts";
import { handleAction } from "../../lib/errors.ts";
import { printPaginated, resolvePageSize } from "../../lib/output.ts";
import { extractTitle } from "../../notion/properties.ts";

const USAGE_TEXT = `agent-notion search — Search Notion by title (NOT full-text content search)

USAGE:
  search <query> [--filter page|database] [--limit <n>] [--cursor <cursor>]

IMPORTANT: Search only matches page and database TITLES. It does NOT search page content.
  To search within content: use "database query <id>" with property filters.
  To find text in page bodies: use "block list <page-id>" and search the output.

OUTPUT:
  { "items": [{ id, type, title, url, parent, lastEditedAt }], "pagination"?: ... }

  type: "page" or "database"
  parent: { type: "database"|"page"|"workspace", id? }

EXAMPLES:
  search "Project Roadmap"                   Search all titles
  search "Task" --filter database            Only databases
  search "Meeting Notes" --filter page       Only pages
  search "Q1" --limit 5                      Limit results
`;

export function registerSearchCommand(program: Command): void {
  program
    .command("search")
    .description("Search Notion by title (pages and databases)")
    .argument("<query>", "Search text (matched against titles)")
    .option("--filter <type>", "Filter by type: page | database")
    .option("--limit <n>", "Max results")
    .option("--cursor <cursor>", "Pagination cursor")
    .option("--usage", "Print detailed search documentation")
    .action(async (query: string, opts: Record<string, string | boolean | undefined>) => {
      if (opts.usage) {
        console.log(USAGE_TEXT.trim());
        return;
      }

      await handleAction(async () => {
        const searchParams: Record<string, unknown> = {
          query,
          page_size: resolvePageSize(opts as Record<string, string | undefined>),
        };
        if (opts.cursor) {
          searchParams.start_cursor = opts.cursor;
        }
        if (opts.filter === "page" || opts.filter === "database") {
          searchParams.filter = { property: "object", value: opts.filter };
        }

        const results = await withAutoRefresh((client) =>
          client.search(searchParams as Parameters<typeof client.search>[0]),
        );

        const items = results.results.map((item: Record<string, unknown>) => {
          const obj = item as {
            id: string;
            object: string;
            url: string;
            parent?: Record<string, unknown>;
            last_edited_time?: string;
            properties?: Record<string, unknown>;
            title?: Array<{ plain_text: string }>;
          };

          let title = "";
          if (obj.object === "page" && obj.properties) {
            title = extractTitle(obj.properties as Parameters<typeof extractTitle>[0]);
          } else if (obj.object === "database" && obj.title) {
            title = obj.title.map((t) => t.plain_text).join("");
          }

          const parent = formatParent(obj.parent);

          return {
            id: obj.id,
            type: obj.object,
            title,
            url: obj.url,
            parent,
            lastEditedAt: obj.last_edited_time,
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
  if (parent.type === "database_id") return { type: "database", id: parent.database_id as string };
  if (parent.type === "page_id") return { type: "page", id: parent.page_id as string };
  if (parent.type === "workspace") return { type: "workspace" };
  return undefined;
}
