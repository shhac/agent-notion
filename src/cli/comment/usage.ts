import type { Command } from "commander";

const USAGE_TEXT = `agent-notion comment — Page and inline comments

SUBCOMMANDS:
  comment list <page-id> [--limit] [--cursor]     List comments on a page
  comment page <page-id> <body>                    Add a page-level comment
  comment inline <block-id> <body> --text <target> [--occurrence <n>]
                                                   Add an inline comment on specific text

INLINE COMMENTS:
  Inline comments are anchored to a specific text substring within a block.
  --text is required and specifies the target text to annotate.
  --occurrence selects which match when the text appears multiple times (default: 1).

  Example:
    comment inline <block-id> "Great point!" --text "hello"
    # Adds a comment on the word "hello" within the block

    comment inline <block-id> "Second one" --text "the" --occurrence 2
    # Adds a comment on the second occurrence of "the"

LIST OUTPUT:
  { "items": [{ id, body, author: { id, name }, createdAt }], "pagination"?: ... }

PAGE/INLINE OUTPUT:
  { id, discussionId, body, createdAt }

LIMITATIONS:
  Inline comments require the v3 backend (desktop session).
  Discussion threads are not supported — all comments are top-level.
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
