/**
 * V3 operation builders for saveTransactions.
 * Constructs the pointer + path + command + args shapes
 * needed by the Notion v3 internal API.
 */

// --- Types ---

export type V3Pointer = {
  table: string;
  id: string;
  spaceId: string;
};

export type V3Operation = {
  pointer: V3Pointer;
  path: string[];
  command: string;
  args: unknown;
};

// --- Pointer helper ---

function ptr(table: string, id: string, spaceId: string): V3Pointer {
  return { table, id, spaceId };
}

function blockPtr(id: string, spaceId: string): V3Pointer {
  return ptr("block", id, spaceId);
}

// --- Low-level operation builders ---

/** Set a value at a specific path (or entire record if path is []). */
function setOp(pointer: V3Pointer, path: string[], args: unknown): V3Operation {
  return { pointer, path, command: "set", args };
}

/** Shallow-merge args into the record root. */
function updateOp(pointer: V3Pointer, args: unknown): V3Operation {
  return { pointer, path: [], command: "update", args };
}

/** Append a child ID to a list (e.g. content). */
function listAfterOp(pointer: V3Pointer, listPath: string, childId: string, afterId?: string): V3Operation {
  const args: Record<string, string> = { id: childId };
  if (afterId) args.after = afterId;
  return { pointer, path: [listPath], command: "listAfter", args };
}

/** Remove a child ID from a list. */
function listRemoveOp(pointer: V3Pointer, listPath: string, childId: string): V3Operation {
  return { pointer, path: [listPath], command: "listRemove", args: { id: childId } };
}

/** Build an update-metadata operation (last_edited_time, last_edited_by). */
function editMetaOp(pointer: V3Pointer, userId: string): V3Operation {
  return updateOp(pointer, {
    last_edited_time: Date.now(),
    last_edited_by_table: "notion_user",
    last_edited_by_id: userId,
  });
}

// --- High-level builders ---

type CreateBlockParams = {
  id: string;
  type: string;
  parentId: string;
  parentTable: string;
  spaceId: string;
  userId: string;
  properties?: Record<string, unknown>;
  format?: Record<string, unknown>;
};

/** Operations to create a new block and add it to a parent's content list. */
export function createBlockOps(params: CreateBlockParams): V3Operation[] {
  const now = Date.now();
  const bp = blockPtr(params.id, params.spaceId);
  const pp = ptr(params.parentTable === "collection" ? "block" : params.parentTable, params.parentId, params.spaceId);

  const blockArgs: Record<string, unknown> = {
    type: params.type,
    id: params.id,
    version: 0,
    created_time: now,
    last_edited_time: now,
    parent_id: params.parentId,
    parent_table: params.parentTable,
    alive: true,
    created_by_table: "notion_user",
    created_by_id: params.userId,
    last_edited_by_table: "notion_user",
    last_edited_by_id: params.userId,
    space_id: params.spaceId,
  };

  if (params.properties) blockArgs.properties = params.properties;
  if (params.format) blockArgs.format = params.format;

  return [
    setOp(bp, [], blockArgs),
    listAfterOp(pp, "content", params.id),
    editMetaOp(pp, params.userId),
  ];
}

/** Operations to archive (soft-delete) a block. */
export function archiveBlockOps(params: {
  id: string;
  parentId: string;
  parentTable: string;
  spaceId: string;
  userId: string;
}): V3Operation[] {
  const bp = blockPtr(params.id, params.spaceId);
  const pp = ptr(params.parentTable, params.parentId, params.spaceId);

  return [
    updateOp(bp, {
      alive: false,
      last_edited_time: Date.now(),
      last_edited_by_table: "notion_user",
      last_edited_by_id: params.userId,
    }),
    listRemoveOp(pp, "content", params.id),
    editMetaOp(pp, params.userId),
  ];
}

/** Operations to update properties on a block via path-based set. */
export function updatePropertyOps(params: {
  id: string;
  spaceId: string;
  userId: string;
  properties?: Record<string, unknown>;
  format?: Record<string, unknown>;
}): V3Operation[] {
  const bp = blockPtr(params.id, params.spaceId);
  const ops: V3Operation[] = [];

  if (params.properties) {
    for (const [key, value] of Object.entries(params.properties)) {
      ops.push(setOp(bp, ["properties", key], value));
    }
  }

  if (params.format) {
    for (const [key, value] of Object.entries(params.format)) {
      ops.push(setOp(bp, ["format", key], value));
    }
  }

  ops.push(editMetaOp(bp, params.userId));
  return ops;
}

// --- Comment/Discussion operation builders ---

type CreateCommentParams = {
  discussionId: string;
  commentId: string;
  pageId: string;
  spaceId: string;
  userId: string;
  text: string;
};

/** Operations to create a new discussion + comment on a page block. */
export function createCommentOps(params: CreateCommentParams): V3Operation[] {
  const now = Date.now();
  const dp = ptr("discussion", params.discussionId, params.spaceId);
  const cp = ptr("comment", params.commentId, params.spaceId);
  const bp = blockPtr(params.pageId, params.spaceId);

  return [
    // 1. Create discussion record
    setOp(dp, [], {
      id: params.discussionId,
      version: 0,
      parent_id: params.pageId,
      parent_table: "block",
      resolved: false,
      comments: [],
      space_id: params.spaceId,
      alive: true,
    }),
    // 2. Link discussion to page block
    listAfterOp(bp, "discussions", params.discussionId),
    // 3. Create comment record
    setOp(cp, [], {
      id: params.commentId,
      version: 0,
      parent_id: params.discussionId,
      parent_table: "discussion",
      text: [[params.text]],
      created_by_table: "notion_user",
      created_by_id: params.userId,
      alive: true,
      space_id: params.spaceId,
    }),
    // 4. Link comment to discussion
    listAfterOp(dp, "comments", params.commentId),
    // 5. Set timestamps
    setOp(cp, ["created_time"], now),
    setOp(cp, ["last_edited_time"], now),
  ];
}

type CreateInlineCommentParams = {
  discussionId: string;
  commentId: string;
  blockId: string;
  spaceId: string;
  userId: string;
  text: string;
  updatedTitle: unknown; // V3RichText with ["m", discussionId] injected
};

/** Operations to create an inline comment anchored to specific text within a block. */
export function createInlineCommentOps(params: CreateInlineCommentParams): V3Operation[] {
  const now = Date.now();
  const dp = ptr("discussion", params.discussionId, params.spaceId);
  const cp = ptr("comment", params.commentId, params.spaceId);
  const bp = blockPtr(params.blockId, params.spaceId);

  return [
    // 1. Create discussion record (parent = the text block, not the page)
    setOp(dp, [], {
      id: params.discussionId,
      version: 0,
      parent_id: params.blockId,
      parent_table: "block",
      resolved: false,
      comments: [],
      space_id: params.spaceId,
      alive: true,
    }),
    // 2. Link discussion to the text block's discussions list
    listAfterOp(bp, "discussions", params.discussionId),
    // 3. Create comment record
    setOp(cp, [], {
      id: params.commentId,
      version: 0,
      parent_id: params.discussionId,
      parent_table: "discussion",
      text: [[params.text]],
      created_by_table: "notion_user",
      created_by_id: params.userId,
      alive: true,
      space_id: params.spaceId,
    }),
    // 4. Link comment to discussion
    listAfterOp(dp, "comments", params.commentId),
    // 5. Set timestamps on comment
    setOp(cp, ["created_time"], now),
    setOp(cp, ["last_edited_time"], now),
    // 6. Update the block's title with the ["m", discussionId] decoration
    setOp(bp, ["properties", "title"], params.updatedTitle),
    // 7. Update block edit metadata
    editMetaOp(bp, params.userId),
  ];
}

// --- Block type mapping (official → v3) ---

const OFFICIAL_TO_V3_TYPE: Record<string, string> = {
  paragraph: "text",
  heading_1: "header",
  heading_2: "sub_header",
  heading_3: "sub_sub_header",
  bulleted_list_item: "bulleted_list",
  numbered_list_item: "numbered_list",
  to_do: "to_do",
  toggle: "toggle",
  code: "code",
  quote: "quote",
  callout: "callout",
  divider: "divider",
  image: "image",
  bookmark: "bookmark",
  equation: "equation",
  embed: "embed",
  video: "video",
  pdf: "pdf",
  audio: "audio",
  file: "file",
};

/**
 * Convert an official API block object into v3 block creation args.
 * Handles the block types produced by markdownToBlocks().
 */
export function officialBlockToV3Args(block: Record<string, unknown>): {
  type: string;
  properties?: Record<string, unknown>;
  format?: Record<string, unknown>;
} {
  const officialType = block.type as string;
  const v3Type = OFFICIAL_TO_V3_TYPE[officialType] ?? officialType;

  const typeData = block[officialType] as Record<string, unknown> | undefined;
  if (!typeData) {
    return { type: v3Type };
  }

  const properties: Record<string, unknown> = {};
  const format: Record<string, unknown> = {};

  // Extract rich_text → title property
  const richText = typeData.rich_text as Array<{ text?: { content: string } }> | undefined;
  if (richText?.length) {
    const text = richText.map((rt) => rt.text?.content ?? "").join("");
    properties.title = [[text]];
  }

  // Type-specific fields
  switch (officialType) {
    case "code": {
      const lang = typeData.language as string | undefined;
      if (lang) properties.language = [[lang]];
      break;
    }
    case "to_do": {
      const checked = typeData.checked as boolean | undefined;
      if (checked) properties.checked = [["Yes"]];
      break;
    }
    case "image":
    case "video":
    case "pdf":
    case "audio":
    case "file":
    case "embed": {
      const external = typeData.external as Record<string, unknown> | undefined;
      const url = (typeData.url ?? external?.url) as string | undefined;
      if (url) properties.source = [[url]];
      break;
    }
    case "bookmark": {
      const url = typeData.url as string | undefined;
      if (url) properties.link = [[url]];
      break;
    }
    case "equation": {
      const expression = typeData.expression as string | undefined;
      if (expression) properties.title = [[expression]];
      break;
    }
    case "callout": {
      const icon = typeData.icon as { emoji?: string } | undefined;
      if (icon?.emoji) format.page_icon = icon.emoji;
      break;
    }
    case "divider":
      break;
  }

  return {
    type: v3Type,
    ...(Object.keys(properties).length > 0 ? { properties } : {}),
    ...(Object.keys(format).length > 0 ? { format } : {}),
  };
}
