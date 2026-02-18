import type { Command } from "commander";

const USAGE_TEXT = `agent-notion — Notion CLI for humans and LLMs (JSON output, LLM-friendly)

COMMANDS:
  search <query> [--filter page|database]    Title-only search (NOT full-text content)

  database list                              List accessible databases
  database get <id>                          Database metadata + properties
  database query <id> [--filter] [--sort]    Query database rows
  database schema <id>                       Property definitions (for building filters)

  page get <id> [--content]                  Page properties + optional content (markdown)
  page create --parent <id> --title <t>      Create page
  page update <id> [--title] [--properties]  Update properties
  page archive <id>                          Move to trash

  block list <page-id>                       Page content as markdown
  block append <page-id> --content <md>      Append content

  comment list <page-id>                     List comments
  comment page <page-id> <body>              Add page-level comment
  comment inline <block-id> <body> --text <t>  Add inline comment on text

  export page <id> [--format] [--recursive]  Export page to file
  export workspace [--format] [--output]     Export entire workspace

  backlinks <page-id>                        Pages that link to a given page
  history <page-id> [--limit <n>]            Version history for a page
  activity [--page <id>] [--limit <n>]       Recent workspace or page activity
  (v3 commands above require: auth import-desktop)

  user list                                  Workspace users
  user me                                    Bot user identity

  config get|set|reset|list-keys             Persistent settings

IDS: UUIDs (with or without dashes).

SEARCH: Title-only. For content search, use database query with filters.
        For page body text, use block list and search the output.

PAGINATION: --limit <n> --cursor <cursor>
  { "items": [...], "pagination": { "hasMore": true, "nextCursor": "..." } }

OUTPUT: JSON to stdout. Errors: { "error": "..." } to stderr with valid values.

TRUNCATION: description/body/content truncated to ~200 chars + companion *Length field.
  --expand <field,...>  Expand specific    --full  Expand all
  Defaults: config set truncation.maxLength|pagination.defaultPageSize <n>

API LIMITS: 3 req/s average. File URLs expire after ~1 hour. Max 1000 blocks/request.

AUTH: Set NOTION_API_KEY env var, or: agent-notion auth login --token <key>
  Multiple workspaces supported. Switch: agent-notion auth workspace switch <alias>

DETAIL: Run "<command> usage" for detailed per-command docs (e.g. "database usage").
`;

export function registerUsageCommand(program: Command): void {
  program
    .command("usage")
    .description("Print concise documentation (LLM-optimized)")
    .action(() => {
      console.log(USAGE_TEXT.trim());
    });
}
