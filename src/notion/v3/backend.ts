/**
 * V3 backend — implements NotionBackend using the v3 internal API.
 * Reads and writes are fully supported via saveTransactions.
 * Comments use discussion/comment records via loadPageChunk + syncRecordValues + submitTransaction.
 */
import type { NotionBackend } from "../interface.ts";
import type {
  Paginated,
  SearchResult,
  DatabaseListItem,
  DatabaseDetail,
  DatabaseSchema,
  QueryRow,
  PageDetail,
  PageCreateResult,
  PageUpdateResult,
  PageArchiveResult,
  NormalizedBlock,
  BlockListResult,
  CommentItem,
  CommentCreateResult,
  UserItem,
  UserMe,
} from "../types.ts";
import { V3HttpClient } from "./client.ts";
import type { V3Block, RecordMap } from "./client.ts";
import {
  transformV3SearchResult,
  transformV3DatabaseListItem,
  transformV3DatabaseDetail,
  transformV3DatabaseSchema,
  transformV3QueryRow,
  transformV3PageDetail,
  transformV3Comment,
  transformV3User,
  transformV3UserMe,
  normalizeV3Block,
  getBlock,
  getCollection,
  getDiscussion,
  getComment,
  getUser,
  getAllBlocks,
  getFirstCollection,
  getFirstCollectionViewId,
  getFirstUser,
  getAllUsers,
  toV3RichText,
  buildV3PropertyValue,
} from "./transforms.ts";
import {
  createBlockOps,
  archiveBlockOps,
  updatePropertyOps,
  createCommentOps,
  officialBlockToV3Args,
} from "./operations.ts";
import type { V3Operation } from "./operations.ts";

export class V3Backend implements NotionBackend {
  readonly kind = "v3" as const;

  constructor(private http: V3HttpClient) {}

  // --- Search ---

  async search(params: {
    query: string;
    filter?: "page" | "database";
    limit?: number;
    cursor?: string;
  }): Promise<Paginated<SearchResult>> {
    const result = await this.http.search({
      query: params.query,
      limit: params.limit ?? 20,
    });

    const items: SearchResult[] = [];
    for (const hit of result.results) {
      const block = getBlock(result.recordMap, hit.id);
      if (!block) continue;

      // Apply page filter (v3 search doesn't have a direct page-only filter)
      if (params.filter === "page" && (block.type === "collection_view_page" || block.type === "collection_view")) {
        continue;
      }
      if (params.filter === "database" && block.type !== "collection_view_page" && block.type !== "collection_view") {
        continue;
      }

      items.push(transformV3SearchResult(block));
    }

    return {
      items,
      hasMore: items.length < result.total,
      nextCursor: undefined, // v3 search doesn't use cursor pagination
    };
  }

  // --- Databases ---

  async listDatabases(params?: {
    limit?: number;
    cursor?: string;
  }): Promise<Paginated<DatabaseListItem>> {
    // Use search to find collection_view_page blocks
    const result = await this.http.search({
      query: "",
      limit: params?.limit ?? 50,
    });

    const items: DatabaseListItem[] = [];
    for (const hit of result.results) {
      const block = getBlock(result.recordMap, hit.id);
      if (!block) continue;
      if (block.type !== "collection_view_page" && block.type !== "collection_view") continue;

      // Find the collection for this collection_view_page
      const collectionId = (block as V3Block & { collection_id?: string }).collection_id;
      const collection = collectionId
        ? getCollection(result.recordMap, collectionId)
        : getFirstCollection(result.recordMap);

      if (collection) {
        items.push(transformV3DatabaseListItem(collection, block.id));
      }
    }

    return {
      items,
      hasMore: false,
      nextCursor: undefined,
    };
  }

  async getDatabase(id: string): Promise<DatabaseDetail> {
    // Load the page to get the collection
    const { recordMap } = await this.http.loadPageChunk({ pageId: id, limit: 1 });
    const collection = await this.resolveCollection(id, recordMap);

    if (!collection) {
      throw new Error(`Database not found: ${id}`);
    }

    return transformV3DatabaseDetail(collection, id);
  }

  async queryDatabase(params: {
    id: string;
    filter?: unknown;
    sort?: unknown;
    limit?: number;
    cursor?: string;
  }): Promise<Paginated<QueryRow>> {
    // First load the page to get collection ID and view ID
    const { recordMap: pageRecordMap } = await this.http.loadPageChunk({ pageId: params.id, limit: 1 });
    const collection = await this.resolveCollection(params.id, pageRecordMap);

    if (!collection) {
      throw new Error(`Database not found: ${params.id}`);
    }

    // Find a collection view ID
    const block = getBlock(pageRecordMap, params.id);
    const viewIds = (block as V3Block & { view_ids?: string[] })?.view_ids;
    const viewId = viewIds?.[0] ?? getFirstCollectionViewId(pageRecordMap);

    if (!viewId) {
      throw new Error(`No view found for database: ${params.id}`);
    }

    // Query the collection
    const result = await this.http.queryCollection({
      collectionId: collection.id,
      collectionViewId: viewId,
      query: params.filter || params.sort
        ? { filter: params.filter as Record<string, unknown>, sort: params.sort as Record<string, unknown> }
        : undefined,
      limit: params.limit ?? 50,
    });

    const items: QueryRow[] = [];
    for (const blockId of result.result.blockIds) {
      const rowBlock = getBlock(result.recordMap, blockId);
      if (!rowBlock) continue;
      items.push(transformV3QueryRow(rowBlock, collection.schema));
    }

    return {
      items,
      hasMore: items.length < result.result.total,
      nextCursor: undefined, // v3 queryCollection doesn't use cursor pagination
    };
  }

  async getDatabaseSchema(id: string): Promise<DatabaseSchema> {
    const { recordMap } = await this.http.loadPageChunk({ pageId: id, limit: 1 });
    const collection = await this.resolveCollection(id, recordMap);

    if (!collection) {
      throw new Error(`Database not found: ${id}`);
    }

    return transformV3DatabaseSchema(collection, id);
  }

  // --- Pages ---

  async getPage(id: string): Promise<PageDetail> {
    const { recordMap } = await this.http.loadPageChunk({ pageId: id, limit: 1 });
    const block = getBlock(recordMap, id);

    if (!block) {
      throw new Error(`Page not found: ${id}`);
    }

    // If the page is a database row, resolve its collection schema for property names
    let schema: Record<string, import("./client.ts").V3PropertySchema> | undefined;
    if (block.parent_table === "collection") {
      const collection = getCollection(recordMap, block.parent_id);
      schema = collection?.schema;
    }

    return transformV3PageDetail(block, schema);
  }

  async createPage(params: {
    parentId: string;
    title: string;
    properties?: Record<string, unknown>;
    icon?: string;
  }): Promise<PageCreateResult> {
    const spaceId = this.http.spaceId_;
    const userId = this.http.userId_;
    const newPageId = crypto.randomUUID();

    // Detect parent type and resolve collection for database parents
    const isDb = await this.isDatabase(params.parentId);
    let parentTable: string;
    let parentId: string;
    const v3Props: Record<string, unknown> = {
      title: toV3RichText(params.title),
    };

    if (isDb) {
      // Database parent: resolve collection for schema-based property mapping
      const { recordMap } = await this.http.loadPageChunk({ pageId: params.parentId, limit: 1 });
      const collection = await this.resolveCollection(params.parentId, recordMap);
      if (!collection) throw new Error(`Could not resolve database: ${params.parentId}`);

      parentTable = "collection";
      parentId = collection.id;

      // Map human-readable property names to schema IDs
      if (params.properties) {
        const schema = collection.schema ?? {};
        for (const [name, value] of Object.entries(params.properties)) {
          if (name === "Name" || name === "title") continue;
          const schemaEntry = Object.entries(schema).find(([, s]) => s.name === name);
          if (schemaEntry) {
            v3Props[schemaEntry[0]] = buildV3PropertyValue(value, schemaEntry[1].type);
          }
        }
      }
    } else {
      parentTable = "block";
      parentId = params.parentId;
    }

    const format: Record<string, unknown> = {};
    if (params.icon) format.page_icon = params.icon;

    const ops = createBlockOps({
      id: newPageId,
      type: "page",
      parentId,
      parentTable,
      spaceId,
      userId,
      properties: v3Props,
      ...(Object.keys(format).length > 0 ? { format } : {}),
    });

    // For database parents, listAfter targets the collection_view_page block, not the collection
    if (isDb) {
      const listAfterOp = ops.find((op) => op.command === "listAfter");
      if (listAfterOp) {
        listAfterOp.pointer = { table: "block", id: params.parentId, spaceId };
      }
      const editMetaOp = ops.find((op) => op.command === "update" && op.pointer.id !== newPageId);
      if (editMetaOp) {
        editMetaOp.pointer = { table: "block", id: params.parentId, spaceId };
      }
    }

    await this.http.saveTransactions(ops);

    const url = `https://www.notion.so/${newPageId.replace(/-/g, "")}`;
    return {
      id: newPageId,
      url,
      title: params.title,
      parent: isDb
        ? { type: "database_id", database_id: params.parentId }
        : { type: "page_id", page_id: params.parentId },
      createdAt: new Date().toISOString(),
    };
  }

  async updatePage(params: {
    id: string;
    title?: string;
    properties?: Record<string, unknown>;
    icon?: string;
  }): Promise<PageUpdateResult> {
    const spaceId = this.http.spaceId_;
    const userId = this.http.userId_;

    const v3Props: Record<string, unknown> = {};
    const v3Format: Record<string, unknown> = {};

    if (params.title) {
      v3Props.title = toV3RichText(params.title);
    }

    if (params.icon) {
      v3Format.page_icon = params.icon;
    }

    // If properties are provided, resolve schema for database rows
    if (params.properties) {
      const { recordMap } = await this.http.loadPageChunk({ pageId: params.id, limit: 1 });
      const block = getBlock(recordMap, params.id);

      if (block?.parent_table === "collection") {
        const collection = getCollection(recordMap, block.parent_id);
        const schema = collection?.schema ?? {};

        for (const [name, value] of Object.entries(params.properties)) {
          if (name === "Name" || name === "title") continue;
          const schemaEntry = Object.entries(schema).find(([, s]) => s.name === name);
          if (schemaEntry) {
            v3Props[schemaEntry[0]] = buildV3PropertyValue(value, schemaEntry[1].type);
          }
        }
      }
    }

    const ops = updatePropertyOps({
      id: params.id,
      spaceId,
      userId,
      ...(Object.keys(v3Props).length > 0 ? { properties: v3Props } : {}),
      ...(Object.keys(v3Format).length > 0 ? { format: v3Format } : {}),
    });

    await this.http.saveTransactions(ops);

    const url = `https://www.notion.so/${params.id.replace(/-/g, "")}`;
    return {
      id: params.id,
      url,
      lastEditedAt: new Date().toISOString(),
    };
  }

  async archivePage(id: string): Promise<PageArchiveResult> {
    const spaceId = this.http.spaceId_;
    const userId = this.http.userId_;

    // Fetch the page to get parent info for listRemove
    const { recordMap } = await this.http.loadPageChunk({ pageId: id, limit: 1 });
    const block = getBlock(recordMap, id);
    if (!block) throw new Error(`Page not found: ${id}`);

    const ops = archiveBlockOps({
      id,
      parentId: block.parent_id,
      parentTable: block.parent_table,
      spaceId,
      userId,
    });

    await this.http.saveTransactions(ops);

    return { id, archived: true };
  }

  // --- Blocks ---

  async listBlocks(params: {
    id: string;
    limit?: number;
    cursor?: string;
  }): Promise<Paginated<NormalizedBlock>> {
    const { recordMap } = await this.http.loadPageChunk({
      pageId: params.id,
      limit: params.limit ?? 50,
    });

    // Get the parent block to find its children list
    const parentBlock = getBlock(recordMap, params.id);
    const childIds = parentBlock?.content ?? [];

    const items: NormalizedBlock[] = [];
    for (const childId of childIds) {
      const childBlock = getBlock(recordMap, childId);
      if (!childBlock || !childBlock.alive) continue;
      items.push(normalizeV3Block(childBlock));
    }

    return {
      items,
      hasMore: false,
      nextCursor: undefined,
    };
  }

  async getAllBlocks(id: string): Promise<BlockListResult> {
    const blocks: NormalizedBlock[] = [];
    let cursor: { stack: unknown[] } | undefined;
    let chunkNumber = 0;

    while (blocks.length < 1000) {
      const result = await this.http.loadPageChunk({
        pageId: id,
        limit: 100,
        cursor,
        chunkNumber,
      });

      // Get the parent block's content list
      const parentBlock = getBlock(result.recordMap, id);
      const childIds = parentBlock?.content ?? [];

      for (const childId of childIds) {
        const childBlock = getBlock(result.recordMap, childId);
        if (!childBlock || !childBlock.alive) continue;
        // Avoid duplicates
        if (!blocks.some((b) => b.id === childBlock.id)) {
          blocks.push(normalizeV3Block(childBlock));
        }
      }

      // Also include any blocks in the recordMap that are children of child blocks
      const allBlocksInMap = getAllBlocks(result.recordMap);
      for (const block of allBlocksInMap) {
        if (block.id === id) continue; // Skip the parent itself
        if (!blocks.some((b) => b.id === block.id)) {
          // Check if this block is a descendant (its parent is in our set or is the root)
          if (childIds.includes(block.id) || block.parent_id === id) {
            blocks.push(normalizeV3Block(block));
          }
        }
      }

      // Check if there's more content
      const hasMoreChunks = result.cursor?.stack?.length > 0;
      if (!hasMoreChunks) break;
      if (blocks.length >= 1000) break;

      cursor = result.cursor;
      chunkNumber++;
    }

    return {
      blocks,
      hasMore: blocks.length >= 1000,
    };
  }

  async getChildBlocks(blockIds: string[]): Promise<Map<string, NormalizedBlock[]>> {
    const childMap = new Map<string, NormalizedBlock[]>();

    // Batch in groups of 5
    for (let i = 0; i < blockIds.length; i += 5) {
      const batch = blockIds.slice(i, i + 5);
      await Promise.all(
        batch.map(async (blockId) => {
          const { blocks } = await this.getAllBlocks(blockId);
          childMap.set(blockId, blocks);
        }),
      );
    }

    return childMap;
  }

  async appendBlocks(params: {
    id: string;
    blocks: unknown[];
  }): Promise<{ blocksAdded: number }> {
    const spaceId = this.http.spaceId_;
    const userId = this.http.userId_;

    const allOps: V3Operation[] = [];

    let previousBlockId: string | undefined;

    for (const block of params.blocks) {
      const blockObj = block as Record<string, unknown>;
      const newBlockId = crypto.randomUUID();
      const { type, properties, format } = officialBlockToV3Args(blockObj);

      const ops = createBlockOps({
        id: newBlockId,
        type,
        parentId: params.id,
        parentTable: "block",
        spaceId,
        userId,
        properties,
        format,
      });

      // If we have a previous block, set the "after" for ordering
      if (previousBlockId) {
        const listAfterOp = ops.find((op) => op.command === "listAfter");
        if (listAfterOp) {
          (listAfterOp.args as Record<string, string>).after = previousBlockId;
        }
      }

      // Skip duplicate editMeta ops for the parent (keep only the last one)
      const nonMetaOps = ops.filter(
        (op) => !(op.command === "update" && op.pointer.id === params.id),
      );
      allOps.push(...nonMetaOps);

      previousBlockId = newBlockId;
    }

    // Add a single editMeta op for the parent at the end
    if (params.blocks.length > 0) {
      allOps.push({
        pointer: { table: "block", id: params.id, spaceId },
        path: [],
        command: "update",
        args: {
          last_edited_time: Date.now(),
          last_edited_by_table: "notion_user",
          last_edited_by_id: userId,
        },
      });
    }

    await this.http.saveTransactions(allOps);

    return { blocksAdded: params.blocks.length };
  }

  // --- Comments (via discussion/comment records) ---

  async listComments(params: {
    pageId: string;
    limit?: number;
    cursor?: string;
  }): Promise<Paginated<CommentItem>> {
    const limit = params.limit ?? 50;

    // Load the page — discussions and comments may be included in the recordMap
    const { recordMap } = await this.http.loadPageChunk({ pageId: params.pageId, limit: 100 });

    // Get discussion IDs from the page block's "discussions" property
    const block = getBlock(recordMap, params.pageId);
    const discussionIds = ((block as V3Block & { discussions?: string[] })?.discussions) ?? [];

    if (discussionIds.length === 0) {
      return { items: [], hasMore: false, nextCursor: undefined };
    }

    // Fetch any discussions/comments not already in the recordMap
    const missingDiscussionIds = discussionIds.filter((id) => !getDiscussion(recordMap, id));
    if (missingDiscussionIds.length > 0) {
      const { recordMap: extraMap } = await this.http.syncRecordValues(
        missingDiscussionIds.map((id) => ({ pointer: { id, table: "discussion" }, version: -1 })),
      );
      this.mergeRecordMap(recordMap, extraMap);
    }

    // Collect all comment IDs from all discussions
    const allCommentIds: string[] = [];
    for (const discId of discussionIds) {
      const disc = getDiscussion(recordMap, discId);
      if (disc?.comments) {
        allCommentIds.push(...disc.comments);
      }
    }

    // Fetch any comments not already in the recordMap
    const missingCommentIds = allCommentIds.filter((id) => !getComment(recordMap, id));
    if (missingCommentIds.length > 0) {
      const { recordMap: extraMap } = await this.http.syncRecordValues(
        missingCommentIds.map((id) => ({ pointer: { id, table: "comment" }, version: -1 })),
      );
      this.mergeRecordMap(recordMap, extraMap);
    }

    // Transform comments, resolving user names from the recordMap
    const items: CommentItem[] = [];
    for (const commentId of allCommentIds) {
      if (items.length >= limit) break;
      const comment = getComment(recordMap, commentId);
      if (!comment || !comment.alive) continue;
      const user = comment.created_by_id ? getUser(recordMap, comment.created_by_id) : undefined;
      items.push(transformV3Comment(comment, user));
    }

    return {
      items,
      hasMore: items.length < allCommentIds.length,
      nextCursor: undefined,
    };
  }

  async addComment(params: {
    pageId: string;
    body: string;
  }): Promise<CommentCreateResult> {
    const discussionId = crypto.randomUUID();
    const commentId = crypto.randomUUID();
    const userId = this.http.userId_;
    const spaceId = this.http.spaceId_;

    const ops = createCommentOps({
      discussionId,
      commentId,
      pageId: params.pageId,
      spaceId,
      userId,
      text: params.body,
    });

    await this.http.saveTransactions(ops);

    return {
      id: commentId,
      body: params.body,
      createdAt: new Date().toISOString(),
    };
  }

  // --- Users ---

  async listUsers(_params?: {
    limit?: number;
    cursor?: string;
  }): Promise<Paginated<UserItem>> {
    const { recordMap } = await this.http.loadUserContent();
    const users = getAllUsers(recordMap);

    return {
      items: users.map(transformV3User),
      hasMore: false,
      nextCursor: undefined,
    };
  }

  async getMe(): Promise<UserMe> {
    const { recordMap } = await this.http.loadUserContent();
    const user = getFirstUser(recordMap);

    if (!user) {
      throw new Error("Could not retrieve user information");
    }

    // Get space name
    let spaceName: string | undefined;
    if (recordMap.space) {
      const firstSpace = Object.values(recordMap.space)[0];
      spaceName = (firstSpace?.value as { name?: string })?.name;
    }

    return transformV3UserMe(user, spaceName);
  }

  // --- Utility ---

  async isDatabase(id: string): Promise<boolean> {
    try {
      const { recordMap } = await this.http.loadPageChunk({ pageId: id, limit: 1 });
      const block = getBlock(recordMap, id);
      return block?.type === "collection_view_page" || block?.type === "collection_view";
    } catch {
      return false;
    }
  }

  // --- Private helpers ---

  /**
   * Resolve the collection (database schema) for a collection_view_page.
   * Looks in the recordMap first, then fetches via syncRecordValues if needed.
   */
  private async resolveCollection(
    pageId: string,
    recordMap: RecordMap,
  ): Promise<import("./client.ts").V3Collection | undefined> {
    // Check if the page block has a collection_id
    const block = getBlock(recordMap, pageId);
    if (!block) return undefined;

    const collectionId = (block as V3Block & { collection_id?: string }).collection_id;

    // Try to find in the existing recordMap
    if (collectionId) {
      const existing = getCollection(recordMap, collectionId);
      if (existing) return existing;

      // Fetch it
      const { recordMap: collRecordMap } = await this.http.syncRecordValues([
        { pointer: { id: collectionId, table: "collection" }, version: 0 },
      ]);
      return getCollection(collRecordMap, collectionId);
    }

    // Fallback: use the first collection in the recordMap
    return getFirstCollection(recordMap);
  }

  /** Merge records from a secondary RecordMap into a primary one. */
  private mergeRecordMap(target: RecordMap, source: RecordMap): void {
    for (const table of Object.keys(source)) {
      const sourceTable = source[table];
      if (!sourceTable) continue;
      if (!target[table]) {
        (target as Record<string, unknown>)[table] = {};
      }
      Object.assign(target[table]!, sourceTable);
    }
  }
}
