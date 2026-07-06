/**
 * Shared block↔markdown conversion. Backend-agnostic — works with NormalizedBlock.
 */
import type { NormalizedBlock } from "./types.ts";

/**
 * Convert a single normalized block to a markdown line.
 */
function blockToMarkdown(block: NormalizedBlock, indent: string = ""): string {
  const text = block.richText;

  switch (block.type) {
    case "paragraph":
      return text ? `${indent}${text}` : "";
    case "heading_1":
      return `${indent}# ${text}`;
    case "heading_2":
      return `${indent}## ${text}`;
    case "heading_3":
      return `${indent}### ${text}`;
    case "bulleted_list_item":
      return `${indent}- ${text}`;
    case "numbered_list_item":
      return `${indent}1. ${text}`;
    case "to_do": {
      const checked = block.checked ? "x" : " ";
      return `${indent}- [${checked}] ${text}`;
    }
    case "toggle":
      return `${indent}> ▶ ${text}`;
    case "code":
      return `${indent}\`\`\`${block.language ?? ""}\n${text}\n${indent}\`\`\``;
    case "quote":
      return `${indent}> ${text}`;
    case "callout":
      return `${indent}> ${block.emoji ?? "💡"} ${text}`;
    case "divider":
      return `${indent}---`;
    case "image":
      return `${indent}![${block.caption || "image"}](${block.url ?? ""})`;
    case "bookmark":
      return `${indent}[${block.caption || block.url || "bookmark"}](${block.url ?? ""})`;
    case "equation":
      return `${indent}$$${block.expression ?? ""}$$`;
    case "child_page":
      return `${indent}📄 ${block.title ?? "Untitled"}`;
    case "child_database":
      return `${indent}📊 ${block.title ?? "Untitled"}`;
    case "table_of_contents":
      return `${indent}[Table of Contents]`;
    case "breadcrumb":
      return `${indent}[Breadcrumb]`;
    case "column_list":
    case "column":
    case "synced_block":
      return "";
    case "link_preview":
      return `${indent}[${block.url ?? "link"}](${block.url ?? ""})`;
    case "embed":
      return `${indent}[embed: ${block.url ?? ""}](${block.url ?? ""})`;
    case "video":
      return `${indent}[video](${block.url ?? ""})`;
    case "pdf":
      return `${indent}[pdf](${block.url ?? ""})`;
    case "audio":
      return `${indent}[audio](${block.url ?? ""})`;
    case "file":
      return `${indent}[${block.caption || block.title || "file"}](${block.url ?? ""})`;
    default:
      return text ? `${indent}${text}` : `${indent}[unsupported: ${block.type}]`;
  }
}

/**
 * Convert an array of normalized blocks to markdown.
 * Uses childBlocksMap for nested content.
 */
export function blocksToMarkdown(
  blocks: NormalizedBlock[],
  childBlocksMap?: Map<string, NormalizedBlock[]>,
  indent: string = "",
): string {
  const lines: string[] = [];

  for (const block of blocks) {
    const line = blockToMarkdown(block, indent);
    if (line !== undefined) {
      lines.push(line);
    }

    if (block.hasChildren && childBlocksMap?.has(block.id)) {
      const children = childBlocksMap.get(block.id)!;
      const childMd = blocksToMarkdown(children, childBlocksMap, indent + "  ");
      if (childMd) {
        lines.push(childMd);
      }
    }
  }

  return lines.filter((l) => l !== "").join("\n\n");
}

/**
 * Flatten a normalized block to a simplified object for --raw output.
 */
export function flattenBlock(block: NormalizedBlock): Record<string, unknown> {
  return {
    id: block.id,
    type: block.type,
    content: block.richText || undefined,
    hasChildren: block.hasChildren,
  };
}

/**
 * Parse markdown text into Notion block objects (for append).
 * Returns blocks in Notion API format (official API compatible).
 */
export function markdownToBlocks(markdown: string): unknown[] {
  const blocks: unknown[] = [];
  const lines = markdown.split("\n");
  let i = 0;

  while (i < lines.length) {
    const line = lines[i]!;

    // Skip empty lines
    if (line.trim() === "") {
      i++;
      continue;
    }

    // Fenced code block
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
        type: "code",
        code: {
          rich_text: [{ type: "text", text: { content: codeLines.join("\n") } }],
          language: lang || "plain text",
        },
      });
      continue;
    }

    // Headings
    const headingMatch = line.match(/^(#{1,3})\s+(.+)$/);
    if (headingMatch) {
      const level = headingMatch[1]!.length as 1 | 2 | 3;
      const type = `heading_${level}` as const;
      blocks.push({
        type,
        [type]: { rich_text: [{ type: "text", text: { content: headingMatch[2]! } }] },
      });
      i++;
      continue;
    }

    // Divider
    if (/^(-{3,}|\*{3,})$/.test(line.trim())) {
      blocks.push({ type: "divider", divider: {} });
      i++;
      continue;
    }

    // Todo
    const todoMatch = line.match(/^[-*]\s+\[([ xX])\]\s+(.+)$/);
    if (todoMatch) {
      blocks.push({
        type: "to_do",
        to_do: {
          rich_text: [{ type: "text", text: { content: todoMatch[2]! } }],
          checked: todoMatch[1] !== " ",
        },
      });
      i++;
      continue;
    }

    // Bulleted list
    const bulletMatch = line.match(/^[-*]\s+(.+)$/);
    if (bulletMatch) {
      blocks.push({
        type: "bulleted_list_item",
        bulleted_list_item: {
          rich_text: [{ type: "text", text: { content: bulletMatch[1]! } }],
        },
      });
      i++;
      continue;
    }

    // Numbered list
    const numberedMatch = line.match(/^\d+\.\s+(.+)$/);
    if (numberedMatch) {
      blocks.push({
        type: "numbered_list_item",
        numbered_list_item: {
          rich_text: [{ type: "text", text: { content: numberedMatch[1]! } }],
        },
      });
      i++;
      continue;
    }

    // Blockquote
    const quoteMatch = line.match(/^>\s+(.+)$/);
    if (quoteMatch) {
      blocks.push({
        type: "quote",
        quote: {
          rich_text: [{ type: "text", text: { content: quoteMatch[1]! } }],
        },
      });
      i++;
      continue;
    }

    // Default: paragraph
    blocks.push({
      type: "paragraph",
      paragraph: {
        rich_text: [{ type: "text", text: { content: line } }],
      },
    });
    i++;
  }

  return blocks;
}
