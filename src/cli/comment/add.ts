import type { Command } from "commander";
import { withBackend } from "../../notion/client.ts";
import { handleAction } from "../../lib/errors.ts";
import { normalizeId } from "../../lib/ids.ts";
import { printJson } from "../../lib/output.ts";

export function registerPage(comment: Command): void {
  comment
    .command("page")
    .description("Add a page-level comment")
    .argument("<page-id>", "Page UUID")
    .argument("<body>", "Comment text")
    .action(async (rawPageId: string, body: string) => {
      const pageId = normalizeId(rawPageId);
      await handleAction(async () => {
        const result = await withBackend((backend) =>
          backend.addComment({ pageId, body }),
        );
        printJson(result);
      });
    });
}

export function registerInline(comment: Command): void {
  comment
    .command("inline")
    .description("Add an inline comment anchored to specific text in a block")
    .argument("<block-id>", "Block UUID containing the target text")
    .argument("<body>", "Comment text")
    .requiredOption("--text <target>", "Text substring to anchor the comment to")
    .option("--occurrence <n>", "Which occurrence if text appears multiple times (default: 1)", "1")
    .action(
      async (
        rawBlockId: string,
        body: string,
        opts: { text: string; occurrence: string },
      ) => {
        const blockId = normalizeId(rawBlockId);
        const occurrence = parseInt(opts.occurrence, 10);
        if (isNaN(occurrence) || occurrence < 1) {
          console.error("Error: --occurrence must be a positive integer");
          process.exit(1);
        }
        await handleAction(async () => {
          const result = await withBackend((backend) =>
            backend.addInlineComment({ blockId, body, text: opts.text, occurrence }),
          );
          printJson({ ...result, anchorText: opts.text });
        });
      },
    );
}
