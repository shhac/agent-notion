/**
 * V3 comments — discussion/comment records via loadPageChunk +
 * syncRecordValues + saveTransactions.
 */
import type { Paginated, CommentItem, CommentCreateResult } from "../types.ts";
import type { V3HttpClient } from "./client.ts";
import type { RecordMap, V3Block, V3Decoration } from "./record-map.ts";
import {
  getBlock,
  getDiscussion,
  getComment,
  getUser,
  mergeRecordMap,
} from "./record-map.ts";
import {
  transformV3Comment,
  v3RichTextToPlain,
  addDecorationToRange,
  extractAnchorText,
} from "./transforms.ts";
import { createCommentOps, createInlineCommentOps } from "./operations.ts";

/**
 * Collect discussion IDs from a page block and its descendant blocks,
 * walking the content tree so only true descendants are considered.
 */
export function collectDiscussionIds(recordMap: RecordMap, pageId: string): string[] {
  const discussionIds: string[] = [];
  const seenDiscussions = new Set<string>();
  const queue: string[] = [pageId];
  const visited = new Set<string>();

  while (queue.length > 0) {
    const blockId = queue.pop()!;
    if (visited.has(blockId)) continue;
    visited.add(blockId);
    const block = getBlock(recordMap, blockId);
    if (!block) continue;

    const blockDiscussions = (block as V3Block & { discussions?: string[] }).discussions ?? [];
    for (const discId of blockDiscussions) {
      if (!seenDiscussions.has(discId)) {
        seenDiscussions.add(discId);
        discussionIds.push(discId);
      }
    }

    queue.push(...(block.content ?? []));
  }

  return discussionIds;
}

/**
 * Build a map of discussionId → anchorText by examining the blocks
 * that discussions are parented to.
 */
export function buildAnchorTextMap(
  recordMap: RecordMap,
  discussionIds: string[],
): Map<string, string> {
  const anchorTextMap = new Map<string, string>();
  for (const discId of discussionIds) {
    const disc = getDiscussion(recordMap, discId);
    // Anchor text only exists for discussions parented to a block
    if (!disc || disc.parent_table !== "block") continue;
    const parentBlock = getBlock(recordMap, disc.parent_id);
    if (!parentBlock?.properties?.title) continue;
    const anchor = extractAnchorText(parentBlock.properties.title, discId);
    if (anchor) {
      anchorTextMap.set(discId, anchor);
    }
  }
  return anchorTextMap;
}

/** Fetch records missing from the RecordMap and merge them in. */
async function fetchMissingRecords(
  http: V3HttpClient,
  recordMap: RecordMap,
  table: "discussion" | "comment",
  ids: string[],
): Promise<void> {
  if (ids.length === 0) return;
  const { recordMap: extraMap } = await http.syncRecordValues(
    ids.map((id) => ({ pointer: { id, table }, version: -1 })),
  );
  mergeRecordMap(recordMap, extraMap);
}

export async function listComments(
  http: V3HttpClient,
  params: {
    pageId: string;
    limit?: number;
    cursor?: string;
  },
): Promise<Paginated<CommentItem>> {
  const limit = params.limit ?? 50;

  // Load the page — discussions and comments may be included in the recordMap
  const { recordMap } = await http.loadPageChunk({ pageId: params.pageId, limit: 100 });

  const discussionIds = collectDiscussionIds(recordMap, params.pageId);
  if (discussionIds.length === 0) {
    return { items: [], hasMore: false, nextCursor: undefined };
  }

  await fetchMissingRecords(
    http,
    recordMap,
    "discussion",
    discussionIds.filter((id) => !getDiscussion(recordMap, id)),
  );

  // Collect all comment IDs from all discussions
  const allCommentIds: string[] = [];
  for (const discId of discussionIds) {
    const disc = getDiscussion(recordMap, discId);
    if (disc?.comments) {
      allCommentIds.push(...disc.comments);
    }
  }

  await fetchMissingRecords(
    http,
    recordMap,
    "comment",
    allCommentIds.filter((id) => !getComment(recordMap, id)),
  );

  const anchorTextMap = buildAnchorTextMap(recordMap, discussionIds);

  // Transform comments, resolving user names and anchor text from the recordMap
  const items: CommentItem[] = [];
  for (const commentId of allCommentIds) {
    if (items.length >= limit) break;
    const comment = getComment(recordMap, commentId);
    if (!comment || !comment.alive) continue;
    const user = comment.created_by_id ? getUser(recordMap, comment.created_by_id) : undefined;
    // A comment's parent_id is the discussion it belongs to
    const anchorText = anchorTextMap.get(comment.parent_id);
    items.push(transformV3Comment(comment, user, anchorText));
  }

  return {
    items,
    hasMore: items.length < allCommentIds.length,
    nextCursor: undefined,
  };
}

export async function addComment(
  http: V3HttpClient,
  params: {
    pageId: string;
    body: string;
  },
): Promise<CommentCreateResult> {
  const discussionId = crypto.randomUUID();
  const commentId = crypto.randomUUID();

  const ops = createCommentOps({
    discussionId,
    commentId,
    pageId: params.pageId,
    spaceId: http.spaceId_,
    userId: http.userId_,
    text: params.body,
  });

  await http.saveTransactions(ops);

  return {
    id: commentId,
    discussionId,
    body: params.body,
    createdAt: new Date().toISOString(),
  };
}

export async function addInlineComment(
  http: V3HttpClient,
  params: {
    blockId: string;
    body: string;
    text: string;
    occurrence?: number;
  },
): Promise<CommentCreateResult> {
  const discussionId = crypto.randomUUID();
  const commentId = crypto.randomUUID();

  // Fetch the block record directly via syncRecordValues (works for any block, not just pages)
  const { recordMap } = await http.syncRecordValues([
    { pointer: { id: params.blockId, table: "block" }, version: -1 },
  ]);
  const block = getBlock(recordMap, params.blockId);
  if (!block) throw new Error(`Block not found: ${params.blockId}`);

  const currentTitle = block.properties?.title;
  if (!currentTitle || currentTitle.length === 0) {
    throw new Error(`Block ${params.blockId} has no text content to annotate.`);
  }

  // Find the target text occurrence in the plain text
  const plainText = v3RichTextToPlain(currentTitle);
  const occurrence = params.occurrence ?? 1;
  let startOffset = -1;
  let found = 0;
  let searchFrom = 0;
  while (found < occurrence) {
    const idx = plainText.indexOf(params.text, searchFrom);
    if (idx === -1) break;
    found++;
    if (found === occurrence) {
      startOffset = idx;
    }
    searchFrom = idx + 1;
  }

  if (startOffset === -1) {
    if (found === 0) {
      throw new Error(`Text '${params.text}' not found in block ${params.blockId}.`);
    }
    throw new Error(`Text '${params.text}' has only ${found} occurrence${found === 1 ? "" : "s"} in this block.`);
  }

  const endOffset = startOffset + params.text.length;

  // Add ["m", discussionId] decoration to the target range
  const decoration: V3Decoration = ["m", discussionId];
  const updatedTitle = addDecorationToRange(currentTitle, startOffset, endOffset, decoration);

  const ops = createInlineCommentOps({
    discussionId,
    commentId,
    blockId: params.blockId,
    spaceId: http.spaceId_,
    userId: http.userId_,
    text: params.body,
    updatedTitle,
  });

  await http.saveTransactions(ops);

  return {
    id: commentId,
    discussionId,
    body: params.body,
    createdAt: new Date().toISOString(),
  };
}
