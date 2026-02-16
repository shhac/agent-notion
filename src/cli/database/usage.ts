import type { Command } from "commander";

const USAGE_TEXT = `agent-notion database — Database exploration and querying

SUBCOMMANDS:
  database list [--limit] [--cursor]           List accessible databases
  database get <id>                            Full metadata + property definitions
  database query <id> [--filter] [--sort]      Query rows (pages) with filters
  database schema <id>                         Compact property definitions (for building filters)

QUERY FILTERS (JSON):
  --filter '{"property":"Status","status":{"equals":"In progress"}}'
  --filter '{"and":[{"property":"Priority","select":{"equals":"High"}},{"property":"Due Date","date":{"before":"2026-03-01"}}]}'
  --filter '{"property":"Tags","multi_select":{"contains":"Bug"}}'

QUERY SORTS (JSON):
  --sort '[{"property":"Due Date","direction":"ascending"}]'
  --sort '[{"timestamp":"last_edited_time","direction":"descending"}]'

SCHEMA OUTPUT: Lists property names, types, and options (for select/multi_select/status).
  Use schema to discover valid property names and values before building filters.

PROPERTY TYPES: title, rich_text, number, select, multi_select, status, date, people,
  checkbox, url, email, phone_number, files, relation, formula, rollup, unique_id,
  created_time, last_edited_time, created_by, last_edited_by

IDS: UUIDs (with or without dashes).
PAGINATION: --limit <n> --cursor <cursor>
`;

export function registerUsage(database: Command): void {
  database
    .command("usage")
    .description("Print detailed database documentation (LLM-optimized)")
    .action(() => {
      console.log(USAGE_TEXT.trim());
    });
}
