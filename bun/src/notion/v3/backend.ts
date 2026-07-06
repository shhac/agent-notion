/**
 * V3 backend — implements NotionBackend using the v3 internal API.
 * Reads and writes are fully supported via saveTransactions.
 * Comments are delegated to comments.ts (discussion/comment records).
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
  PageTrashResult,
  NormalizedBlock,
  BlockListResult,
  BlockUpdateResult,
  BlockDeleteResult,
  BlockMoveResult,
  CommentItem,
  CommentCreateResult,
  UserItem,
  UserMe,
} from "../types.ts";
import { V3HttpClient } from "./client.ts";
import type { V3Block, V3Collection, V3PropertySchema, RecordMap } from "./record-map.ts";
import {
  getBlock,
  getCollection,
  getAllBlocks,
  getFirstCollection,
  getFirstCollectionViewId,
  getFirstUser,
  getFirstSpace,
  getAllUsers,
} from "./record-map.ts";
import {
  transformV3SearchResult,
  transformV3DatabaseListItem,
  transformV3DatabaseDetail,
  transformV3DatabaseSchema,
  transformV3QueryRow,
  transformV3PageDetail,
  transformV3User,
  transformV3UserMe,
  normalizeV3Block,
  toV3RichText,
  mapPropertiesToSchema,
} from "./transforms.ts";
import { listComments, addComment, addInlineComment } from "./comments.ts";
import {
  createBlockOps,
  trashBlockOps,
  archivedPageOps,
  moveBlockOps,
  updatePropertyOps,
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
      hasMore: false,
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
      hasMore: false,
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
    let schema: Record<string, V3PropertySchema> | undefined;
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

    // Detect parent type and resolve collection for database parents (single fetch)
    const { recordMap } = await this.http.loadPageChunk({ pageId: params.parentId, limit: 1 });
    const parentBlock = getBlock(recordMap, params.parentId);
    const isDb = parentBlock?.type === "collection_view_page" || parentBlock?.type === "collection_view";
    let parentTable: string;
    let parentId: string;
    const v3Props: Record<string, unknown> = {
      title: toV3RichText(params.title),
    };

    if (isDb) {
      // Database parent: resolve collection for schema-based property mapping
      const collection = await this.resolveCollection(params.parentId, recordMap);
      if (!collection) throw new Error(`Could not resolve database: ${params.parentId}`);

      parentTable = "collection";
      parentId = collection.id;

      // Map human-readable property names to schema IDs
      if (params.properties) {
        Object.assign(v3Props, mapPropertiesToSchema(params.properties, collection.schema ?? {}));
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
        Object.assign(v3Props, mapPropertiesToSchema(params.properties, collection?.schema ?? {}));
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

  async trashPage(id: string): Promise<PageTrashResult> {
    const spaceId = this.http.spaceId_;
    const userId = this.http.userId_;

    const { recordMap } = await this.http.loadPageChunk({ pageId: id, limit: 1 });
    const block = getBlock(recordMap, id);
    if (!block) throw new Error(`Page not found: ${id}`);

    const ops = trashBlockOps({
      id,
      parentId: block.parent_id,
      parentTable: block.parent_table,
      spaceId,
      userId,
    });

    await this.http.saveTransactions(ops);

    return { id, trashed: true };
  }

  async restorePage(id: string): Promise<PageTrashResult> {
    await this.http.restoreRecord({ id, table: "block" });
    return { id, trashed: false };
  }

  async archivePage(id: string): Promise<PageArchiveResult> {
    const ops = archivedPageOps({
      id,
      spaceId: this.http.spaceId_,
      userId: this.http.userId_,
      archive: true,
    });
    await this.http.saveTransactions(ops);
    return { id, archived: true };
  }

  async unarchivePage(id: string): Promise<PageArchiveResult> {
    const ops = archivedPageOps({
      id,
      spaceId: this.http.spaceId_,
      userId: this.http.userId_,
      archive: false,
    });
    await this.http.saveTransactions(ops);
    return { id, archived: false };
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
    const seenIds = new Set<string>();
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
      const childIdSet = new Set(childIds);

      for (const childId of childIds) {
        const childBlock = getBlock(result.recordMap, childId);
        if (!childBlock || !childBlock.alive) continue;
        // Avoid duplicates
        if (!seenIds.has(childBlock.id)) {
          seenIds.add(childBlock.id);
          blocks.push(normalizeV3Block(childBlock));
        }
      }

      // Also include any blocks in the recordMap that are children of child blocks
      const allBlocksInMap = getAllBlocks(result.recordMap);
      for (const block of allBlocksInMap) {
        if (block.id === id) continue; // Skip the parent itself
        if (!seenIds.has(block.id)) {
          // Check if this block is a descendant (its parent is in our set or is the root)
          if (childIdSet.has(block.id) || block.parent_id === id) {
            seenIds.add(block.id);
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

  async updateBlock(params: {
    id: string;
    content?: string;
    type?: string;
  }): Promise<BlockUpdateResult> {
    const spaceId = this.http.spaceId_;
    const userId = this.http.userId_;

    // Fetch current block to know its type
    const { recordMap } = await this.http.syncRecordValues([
      { pointer: { id: params.id, table: "block" }, version: -1 },
    ]);
    const block = getBlock(recordMap, params.id);
    if (!block) throw new Error(`Block not found: ${params.id}`);

    const v3Props: Record<string, unknown> = {};
    if (params.content !== undefined) {
      v3Props.title = toV3RichText(params.content);
    }

    const ops = updatePropertyOps({
      id: params.id,
      spaceId,
      userId,
      ...(Object.keys(v3Props).length > 0 ? { properties: v3Props } : {}),
    });

    await this.http.saveTransactions(ops);

    return {
      id: params.id,
      lastEditedAt: new Date().toISOString(),
    };
  }

  async deleteBlock(id: string): Promise<BlockDeleteResult> {
    const spaceId = this.http.spaceId_;
    const userId = this.http.userId_;

    // Fetch the block to get parent info for listRemove
    const { recordMap } = await this.http.syncRecordValues([
      { pointer: { id, table: "block" }, version: -1 },
    ]);
    const block = getBlock(recordMap, id);
    if (!block) throw new Error(`Block not found: ${id}`);

    const ops = trashBlockOps({
      id,
      parentId: block.parent_id,
      parentTable: block.parent_table,
      spaceId,
      userId,
    });

    await this.http.saveTransactions(ops);

    return { id, deleted: true };
  }

  async moveBlock(params: {
    id: string;
    parentId?: string;
    afterId?: string;
  }): Promise<BlockMoveResult> {
    const spaceId = this.http.spaceId_;
    const userId = this.http.userId_;

    // Fetch the block to get its current parent
    const { recordMap } = await this.http.syncRecordValues([
      { pointer: { id: params.id, table: "block" }, version: -1 },
    ]);
    const block = getBlock(recordMap, params.id);
    if (!block) throw new Error(`Block not found: ${params.id}`);

    const newParentId = params.parentId ?? block.parent_id;
    const newParentTable = params.parentId ? "block" : block.parent_table;

    const ops = moveBlockOps({
      id: params.id,
      oldParentId: block.parent_id,
      oldParentTable: block.parent_table,
      newParentId,
      newParentTable,
      spaceId,
      userId,
      afterId: params.afterId,
    });

    await this.http.saveTransactions(ops);

    return {
      id: params.id,
      parentId: newParentId,
      afterId: params.afterId,
    };
  }

  // --- Comments (via discussion/comment records) ---

  async listComments(params: {
    pageId: string;
    limit?: number;
    cursor?: string;
  }): Promise<Paginated<CommentItem>> {
    return listComments(this.http, params);
  }

  async addComment(params: {
    pageId: string;
    body: string;
  }): Promise<CommentCreateResult> {
    return addComment(this.http, params);
  }

  async addInlineComment(params: {
    blockId: string;
    body: string;
    text: string;
    occurrence?: number;
  }): Promise<CommentCreateResult> {
    return addInlineComment(this.http, params);
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

    return transformV3UserMe(user, getFirstSpace(recordMap)?.name);
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
  ): Promise<V3Collection | undefined> {
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
}
