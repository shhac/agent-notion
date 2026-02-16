/**
 * Block-to-markdown conversion for Notion page content.
 */

type RichTextItem = {
  plain_text: string;
  annotations?: {
    bold?: boolean;
    italic?: boolean;
    strikethrough?: boolean;
    code?: boolean;
  };
  href?: string | null;
};

type NotionBlock = {
  id: string;
  type: string;
  has_children: boolean;
  [key: string]: unknown;
};

function richTextToPlain(items: RichTextItem[] | undefined): string {
  if (!items || items.length === 0) return "";
  return items.map((t) => t.plain_text).join("");
}

function blockContentText(block: NotionBlock): string {
  const data = block[block.type] as { rich_text?: RichTextItem[] } | undefined;
  return richTextToPlain(data?.rich_text);
}

/**
 * Convert a single block to markdown line(s).
 */
function blockToMarkdown(block: NotionBlock, indent: string = ""): string {
  const text = blockContentText(block);

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
      const data = block.to_do as { checked?: boolean; rich_text?: RichTextItem[] } | undefined;
      const checked = data?.checked ? "x" : " ";
      return `${indent}- [${checked}] ${text}`;
    }
    case "toggle":
      return `${indent}> ▶ ${text}`;
    case "code": {
      const data = block.code as { rich_text?: RichTextItem[]; language?: string } | undefined;
      const lang = data?.language ?? "";
      const code = richTextToPlain(data?.rich_text);
      return `${indent}\`\`\`${lang}\n${code}\n${indent}\`\`\``;
    }
    case "quote":
      return `${indent}> ${text}`;
    case "callout": {
      const data = block.callout as {
        rich_text?: RichTextItem[];
        icon?: { type: string; emoji?: string };
      } | undefined;
      const icon = data?.icon?.emoji ?? "💡";
      return `${indent}> ${icon} ${text}`;
    }
    case "divider":
      return `${indent}---`;
    case "image": {
      const data = block.image as {
        type: string;
        file?: { url: string };
        external?: { url: string };
        caption?: RichTextItem[];
      } | undefined;
      const url = data?.file?.url ?? data?.external?.url ?? "";
      const caption = richTextToPlain(data?.caption) || "image";
      return `${indent}![${caption}](${url})`;
    }
    case "bookmark": {
      const data = block.bookmark as { url?: string; caption?: RichTextItem[] } | undefined;
      const caption = richTextToPlain(data?.caption) || data?.url || "bookmark";
      return `${indent}[${caption}](${data?.url ?? ""})`;
    }
    case "equation": {
      const data = block.equation as { expression?: string } | undefined;
      return `${indent}$$${data?.expression ?? ""}$$`;
    }
    case "child_page": {
      const data = block.child_page as { title?: string } | undefined;
      return `${indent}📄 ${data?.title ?? "Untitled"}`;
    }
    case "child_database": {
      const data = block.child_database as { title?: string } | undefined;
      return `${indent}📊 ${data?.title ?? "Untitled"}`;
    }
    case "table_of_contents":
      return `${indent}[Table of Contents]`;
    case "breadcrumb":
      return `${indent}[Breadcrumb]`;
    case "column_list":
    case "column":
      // Columns are structural — children rendered separately
      return "";
    case "synced_block":
      // Content comes from children
      return "";
    case "link_preview": {
      const data = block.link_preview as { url?: string } | undefined;
      return `${indent}[${data?.url ?? "link"}](${data?.url ?? ""})`;
    }
    case "embed": {
      const data = block.embed as { url?: string } | undefined;
      return `${indent}[embed: ${data?.url ?? ""}](${data?.url ?? ""})`;
    }
    case "video": {
      const data = block.video as {
        type: string;
        file?: { url: string };
        external?: { url: string };
      } | undefined;
      const url = data?.file?.url ?? data?.external?.url ?? "";
      return `${indent}[video](${url})`;
    }
    case "pdf": {
      const data = block.pdf as {
        type: string;
        file?: { url: string };
        external?: { url: string };
      } | undefined;
      const url = data?.file?.url ?? data?.external?.url ?? "";
      return `${indent}[pdf](${url})`;
    }
    case "audio": {
      const data = block.audio as {
        type: string;
        file?: { url: string };
        external?: { url: string };
      } | undefined;
      const url = data?.file?.url ?? data?.external?.url ?? "";
      return `${indent}[audio](${url})`;
    }
    case "file": {
      const data = block.file as {
        type: string;
        file?: { url: string };
        external?: { url: string };
        name?: string;
        caption?: RichTextItem[];
      } | undefined;
      const url = data?.file?.url ?? data?.external?.url ?? "";
      const name = richTextToPlain(data?.caption) || data?.name || "file";
      return `${indent}[${name}](${url})`;
    }
    default:
      return text ? `${indent}${text}` : `${indent}[unsupported: ${block.type}]`;
  }
}

/**
 * Convert a flat array of blocks to markdown.
 * Handles children by recursive fetching (caller provides childBlocks map).
 */
export function blocksToMarkdown(
  blocks: NotionBlock[],
  childBlocksMap?: Map<string, NotionBlock[]>,
  indent: string = "",
): string {
  const lines: string[] = [];

  for (const block of blocks) {
    const line = blockToMarkdown(block, indent);
    if (line !== undefined) {
      lines.push(line);
    }

    // Render children if available
    if (block.has_children && childBlocksMap?.has(block.id)) {
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
 * Flatten a block to a simplified object for --raw output.
 */
export function flattenBlock(block: NotionBlock): Record<string, unknown> {
  return {
    id: block.id,
    type: block.type,
    content: blockContentText(block) || undefined,
    hasChildren: block.has_children,
  };
}
