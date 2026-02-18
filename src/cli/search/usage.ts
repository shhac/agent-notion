import type { Command } from "commander";

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

export function registerUsage(search: Command): void {
  search
    .command("usage")
    .description("Print detailed search documentation (LLM-optimized)")
    .action(() => {
      console.log(USAGE_TEXT.trim());
    });
}
