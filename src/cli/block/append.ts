import type { Command } from "commander";
import { withAutoRefresh } from "../../notion/client.ts";
import { handleAction, CliError } from "../../lib/errors.ts";
import { printJson } from "../../lib/output.ts";

export function registerAppend(block: Command): void {
  block
    .command("append")
    .description("Append blocks to a page")
    .argument("<page-id>", "Page or block UUID")
    .option("--content <markdown>", "Content as markdown")
    .option("--blocks <json>", "Content as Notion block objects (JSON array)")
    .action(async (pageId: string, opts: Record<string, string | undefined>) => {
      await handleAction(async () => {
        if (!opts.content && !opts.blocks) {
          throw new CliError("Provide --content (markdown) or --blocks (JSON array).");
        }

        let children: unknown[];

        if (opts.blocks) {
          try {
            children = JSON.parse(opts.blocks);
            if (!Array.isArray(children)) {
              throw new CliError("--blocks must be a JSON array of block objects.");
            }
          } catch (e) {
            if (e instanceof CliError) throw e;
            throw new CliError(
              "Invalid --blocks JSON. Expected an array of Notion block objects.",
            );
          }
        } else {
          // Convert markdown to blocks
          children = markdownToBlocks(opts.content!);
        }

        const result = await withAutoRefresh((client) =>
          client.blocks.children.append({
            block_id: pageId,
            children: children as Parameters<
              typeof client.blocks.children.append
            >[0]["children"],
          }),
        );

        printJson({
          pageId,
          blocksAdded: (result.results as unknown[]).length,
        });
      });
    });
}

/**
 * Simple markdown to Notion blocks conversion.
 * Handles common markdown patterns.
 */
function markdownToBlocks(markdown: string): Record<string, unknown>[] {
  const lines = markdown.split("\n");
  const blocks: Record<string, unknown>[] = [];
  let i = 0;

  while (i < lines.length) {
    const line = lines[i]!;

    // Skip empty lines
    if (line.trim() === "") {
      i++;
      continue;
    }

    // Fenced code blocks
    if (line.trim().startsWith("```")) {
      const lang = line.trim().slice(3).trim();
      const codeLines: string[] = [];
      i++;
      while (i < lines.length && !lines[i]!.trim().startsWith("```")) {
        codeLines.push(lines[i]!);
        i++;
      }
      i++; // skip closing ```
      blocks.push({
        object: "block",
        type: "code",
        code: {
          rich_text: [{ type: "text", text: { content: codeLines.join("\n") } }],
          language: lang || "plain text",
        },
      });
      continue;
    }

    // Headings
    if (line.startsWith("### ")) {
      blocks.push(heading(3, line.slice(4)));
    } else if (line.startsWith("## ")) {
      blocks.push(heading(2, line.slice(3)));
    } else if (line.startsWith("# ")) {
      blocks.push(heading(1, line.slice(2)));
    }
    // Divider
    else if (line.trim() === "---" || line.trim() === "***") {
      blocks.push({ object: "block", type: "divider", divider: {} });
    }
    // Todo items
    else if (line.trim().startsWith("- [x] ") || line.trim().startsWith("- [ ] ")) {
      const checked = line.trim().startsWith("- [x]");
      const text = line.trim().slice(6);
      blocks.push({
        object: "block",
        type: "to_do",
        to_do: {
          rich_text: [{ type: "text", text: { content: text } }],
          checked,
        },
      });
    }
    // Bulleted list
    else if (line.trim().startsWith("- ") || line.trim().startsWith("* ")) {
      const text = line.trim().slice(2);
      blocks.push({
        object: "block",
        type: "bulleted_list_item",
        bulleted_list_item: {
          rich_text: [{ type: "text", text: { content: text } }],
        },
      });
    }
    // Numbered list
    else if (/^\d+\.\s/.test(line.trim())) {
      const text = line.trim().replace(/^\d+\.\s/, "");
      blocks.push({
        object: "block",
        type: "numbered_list_item",
        numbered_list_item: {
          rich_text: [{ type: "text", text: { content: text } }],
        },
      });
    }
    // Blockquote
    else if (line.trim().startsWith("> ")) {
      const text = line.trim().slice(2);
      blocks.push({
        object: "block",
        type: "quote",
        quote: {
          rich_text: [{ type: "text", text: { content: text } }],
        },
      });
    }
    // Paragraph (default)
    else {
      blocks.push({
        object: "block",
        type: "paragraph",
        paragraph: {
          rich_text: [{ type: "text", text: { content: line } }],
        },
      });
    }

    i++;
  }

  return blocks;
}

function heading(level: number, text: string): Record<string, unknown> {
  const type = `heading_${level}`;
  return {
    object: "block",
    type,
    [type]: {
      rich_text: [{ type: "text", text: { content: text } }],
    },
  };
}
