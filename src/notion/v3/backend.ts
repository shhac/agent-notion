/**
 * V3 backend — implements NotionBackend using the v3 internal API.
 * Reads are fully supported. Writes are TODO (complex submitTransaction).
 * Comments are not available in v3 — methods throw clear errors.
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
  transformV3User,
  transformV3UserMe,
  normalizeV3Block,
  getBlock,
  getCollection,
  getAllBlocks,
  getFirstCollection,
  getFirstCollectionViewId,
  getFirstUser,
  getAllUsers,
} from "./transforms.ts";

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
      filters: params.filter
        ? {
            isDeletedOnly: false,
            excludeTemplates: true,
            navigableBlockContentOnly: true,
            requireEditPermissions: false,
            ...(params.filter === "database"
              ? { type: "collection_view_page" }
              : {}),
          }
        : undefined,
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
      filters: {
        isDeletedOnly: false,
        excludeTemplates: true,
        navigableBlockContentOnly: true,
      },
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

  async createPage(_params: {
    parentId: string;
    title: string;
    properties?: Record<string, unknown>;
    icon?: string;
  }): Promise<PageCreateResult> {
    // TODO: Implement via submitTransaction
    throw new Error("Page creation is not yet supported with v3 backend. Use OAuth authentication for write operations.");
  }

  async updatePage(_params: {
    id: string;
    title?: string;
    properties?: Record<string, unknown>;
    icon?: string;
  }): Promise<PageUpdateResult> {
    // TODO: Implement via submitTransaction
    throw new Error("Page update is not yet supported with v3 backend. Use OAuth authentication for write operations.");
  }

  async archivePage(_id: string): Promise<PageArchiveResult> {
    // TODO: Implement via submitTransaction
    throw new Error("Page archive is not yet supported with v3 backend. Use OAuth authentication for write operations.");
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

  async appendBlocks(_params: {
    id: string;
    blocks: unknown[];
  }): Promise<{ blocksAdded: number }> {
    // TODO: Implement via submitTransaction
    throw new Error("Block append is not yet supported with v3 backend. Use OAuth authentication for write operations.");
  }

  // --- Comments (not available in v3) ---

  async listComments(_params: {
    pageId: string;
    limit?: number;
    cursor?: string;
  }): Promise<Paginated<CommentItem>> {
    throw new Error("Comments are not available with v3 backend. Use OAuth authentication for comment operations.");
  }

  async addComment(_params: {
    pageId: string;
    body: string;
  }): Promise<CommentCreateResult> {
    throw new Error("Comments are not available with v3 backend. Use OAuth authentication for comment operations.");
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
}
