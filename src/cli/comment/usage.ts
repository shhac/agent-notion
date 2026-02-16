import type { Command } from "commander";

const USAGE_TEXT = `agent-notion comment — Page comments

SUBCOMMANDS:
  comment list <page-id> [--limit] [--cursor]   List comments on a page
  comment add <page-id> <body>                   Add a comment to a page

LIST OUTPUT:
  { "items": [{ id, body, author: { id, name }, createdAt }], "pagination"?: ... }

ADD OUTPUT:
  { id, body, createdAt }

LIMITATIONS:
  Comments can only be added to pages (not to specific blocks within a page).
  Discussion threads are not supported — all comments are top-level on the page.
  The API does not support editing or deleting comments.
`;

export function registerUsage(comment: Command): void {
  comment
    .command("usage")
    .description("Print detailed comment documentation (LLM-optimized)")
    .action(() => {
      console.log(USAGE_TEXT.trim());
    });
}
