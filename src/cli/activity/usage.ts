import type { Command } from "commander";

const USAGE_TEXT = `agent-notion activity — Recent workspace or page activity log

USAGE:
  activity [--page <page-id>] [--limit <n>]

  Requires a v3 desktop session (auth import-desktop).

OPTIONS:
  --page <page-id>    Scope to a specific page (UUID or dashless ID)
  --limit <n>         Number of activity entries (default: 20)

OUTPUT:
  { "activities": [{ id, type, pageId, pageTitle, authors, editTypes, startTime, endTime }], "total": <n> }

  type: Activity type (e.g. "page-edited")
  authors: Resolved author names from workspace users
  editTypes: Types of edits within the activity (e.g. "content-change")

EXAMPLES:
  activity                             Recent workspace-wide activity
  activity --page <page-id>            Activity scoped to a specific page
  activity --limit 50                  Fetch more entries
`;

export function registerUsage(activity: Command): void {
  activity
    .command("usage")
    .description("Print detailed activity documentation (LLM-optimized)")
    .action(() => {
      console.log(USAGE_TEXT.trim());
    });
}
