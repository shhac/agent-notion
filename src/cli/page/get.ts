import type { Command } from "commander";
import { withAutoRefresh } from "../../notion/client.ts";
import { handleAction } from "../../lib/errors.ts";
import { printJson } from "../../lib/output.ts";
import { flattenProperties } from "../../notion/properties.ts";
import { blocksToMarkdown, flattenBlock } from "../../notion/blocks.ts";
import type { Client } from "@notionhq/client";

export function registerGet(page: Command): void {
  page
    .command("get")
    .description("Get page properties and optionally content")
    .argument("<page-id>", "Page UUID")
    .option("--content", "Include page content as markdown")
    .option("--raw-content", "Include content as structured block objects")
    .action(async (pageId: string, opts: Record<string, boolean | undefined>) => {
      await handleAction(async () => {
        const result = await withAutoRefresh(async (client) => {
          const page = await client.pages.retrieve({ page_id: pageId });
          const p = page as Record<string, unknown>;

          const output: Record<string, unknown> = {
            id: p.id,
            url: p.url,
            parent: formatParent(p.parent as Record<string, unknown> | undefined),
            properties: flattenProperties(
              p.properties as Parameters<typeof flattenProperties>[0],
            ),
            icon: formatIcon(p.icon as Record<string, unknown> | null),
            createdAt: p.created_time,
            createdBy: formatUser(p.created_by as Record<string, unknown> | undefined),
            lastEditedAt: p.last_edited_time,
            lastEditedBy: formatUser(p.last_edited_by as Record<string, unknown> | undefined),
            archived: p.archived,
          };

          if (opts.content || opts.rawContent) {
            const { blocks, hasMore } = await fetchAllBlocks(client, pageId);

            if (opts.rawContent) {
              output.blocks = blocks.map((b) =>
                flattenBlock(b as Parameters<typeof flattenBlock>[0]),
              );
              output.blockCount = blocks.length;
              if (hasMore) output.contentTruncated = true;
            } else {
              const childMap = await fetchChildBlocks(client, blocks);
              output.content = blocksToMarkdown(
                blocks as Parameters<typeof blocksToMarkdown>[0],
                childMap as Parameters<typeof blocksToMarkdown>[1],
              );
              output.blockCount = blocks.length;
              if (hasMore) output.contentTruncated = true;
            }
          }

          return output;
        });

        printJson(result);
      });
    });
}

async function fetchAllBlocks(
  client: Client,
  blockId: string,
): Promise<{ blocks: Record<string, unknown>[]; hasMore: boolean }> {
  const blocks: Record<string, unknown>[] = [];
  let cursor: string | undefined;
  let hasMore = false;

  // Fetch up to 1000 blocks (API limit)
  while (blocks.length < 1000) {
    const response = await client.blocks.children.list({
      block_id: blockId,
      page_size: 100,
      start_cursor: cursor,
    });

    blocks.push(...(response.results as Record<string, unknown>[]));

    if (!response.has_more) break;
    if (blocks.length >= 1000) {
      hasMore = true;
      break;
    }
    cursor = response.next_cursor ?? undefined;
  }

  return { blocks, hasMore };
}

async function fetchChildBlocks(
  client: Client,
  blocks: Record<string, unknown>[],
): Promise<Map<string, Record<string, unknown>[]>> {
  const childMap = new Map<string, Record<string, unknown>[]>();
  const withChildren = blocks.filter((b) => b.has_children);

  // Fetch children in parallel (limited batch)
  const batches = [];
  for (let i = 0; i < withChildren.length; i += 5) {
    batches.push(withChildren.slice(i, i + 5));
  }

  for (const batch of batches) {
    await Promise.all(
      batch.map(async (block) => {
        const { blocks: children } = await fetchAllBlocks(client, block.id as string);
        childMap.set(block.id as string, children);
      }),
    );
  }

  return childMap;
}

function formatParent(parent: Record<string, unknown> | undefined): Record<string, unknown> | undefined {
  if (!parent) return undefined;
  if (parent.type === "database_id") return { type: "database", id: parent.database_id as string };
  if (parent.type === "page_id") return { type: "page", id: parent.page_id as string };
  if (parent.type === "workspace") return { type: "workspace" };
  return undefined;
}

function formatIcon(icon: Record<string, unknown> | null): unknown {
  if (!icon) return null;
  if (icon.type === "emoji") return { type: "emoji", emoji: icon.emoji };
  if (icon.type === "external") return { type: "external", url: (icon.external as Record<string, unknown>)?.url };
  return null;
}

function formatUser(user: Record<string, unknown> | undefined): unknown {
  if (!user) return null;
  return { id: user.id, name: user.name };
}
