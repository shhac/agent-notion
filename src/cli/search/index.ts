import type { Command } from "commander";
import { withBackend } from "../../notion/client.ts";
import { handleAction } from "../../lib/errors.ts";
import { printPaginated, resolvePageSize } from "../../lib/output.ts";

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
