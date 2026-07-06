import { defineCommand, ExactArgs, type Command } from "../../lib/cli.ts";
import { withBackend } from "../../notion/client.ts";
import { handleAction, CliError } from "../../lib/errors.ts";
import { normalizeId } from "../../lib/ids.ts";
import { printJson } from "../../lib/output.ts";

export function registerPage(comment: Command): void {
  comment.addCommand(
    defineCommand({
      use: "page <page-id> <body>",
      short: "Add a page-level comment",
      args: ExactArgs(2),
      action: async ([rawPageId, body]) => {
        const pageId = normalizeId(rawPageId!);
        await handleAction(async () => {
          const result = await withBackend((backend) =>
            backend.addComment({ pageId, body: body! }),
          );
          printJson(result);
        });
      },
    }),
  );
}

export function registerInline(comment: Command): void {
  comment.addCommand(
    defineCommand({
      use: "inline <block-id> <body>",
      short: "Add an inline comment anchored to specific text in a block",
      args: ExactArgs(2),
      options: {
        text: {
          type: "string",
          required: true,
          description: "Text substring to anchor the comment to",
        },
        occurrence: {
          type: "string",
          default: "1",
          description:
            "Which occurrence if text appears multiple times (default: 1)",
        },
      },
      action: async ([rawBlockId, body], opts) => {
        const blockId = normalizeId(rawBlockId!);
        const occurrence = parseInt(opts.occurrence, 10);
        if (isNaN(occurrence) || occurrence < 1) {
          throw new CliError("--occurrence must be a positive integer");
        }
        await handleAction(async () => {
          const result = await withBackend((backend) =>
            backend.addInlineComment({
              blockId,
              body: body!,
              text: opts.text,
              occurrence,
            }),
          );
          printJson({ ...result, anchorText: opts.text });
        });
      },
    }),
  );
}
