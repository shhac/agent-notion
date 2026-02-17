/**
 * Official SDK backend — wraps @notionhq/client.
 * Implements NotionBackend interface.
 */
import { Client } from "@notionhq/client";
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
import {
  transformSearchResult,
  transformDatabaseListItem,
  transformDatabaseDetail,
  transformDatabaseSchema,
  transformQueryRow,
  transformPageDetail,
  transformComment,
  transformUser,
  normalizeBlock,
} from "./transforms.ts";

export class OfficialBackend implements NotionBackend {
  readonly kind = "official" as const;

  constructor(private client: Client) {}

  // --- Search ---

  async search(params: {
    query: string;
    filter?: "page" | "database";
    limit?: number;
    cursor?: string;
  }): Promise<Paginated<SearchResult>> {
    const searchParams: Record<string, unknown> = {
      query: params.query,
      page_size: params.limit ?? 50,
    };
    if (params.filter) {
      searchParams.filter = { property: "object", value: params.filter };
    }
    if (params.cursor) searchParams.start_cursor = params.cursor;

    const result = await this.client.search(searchParams as Parameters<typeof this.client.search>[0]);
    return {
      items: result.results.map((r) => transformSearchResult(r as Record<string, unknown>)),
      hasMore: result.has_more,
      nextCursor: result.next_cursor ?? undefined,
    };
  }

  // --- Databases ---

  async listDatabases(params?: {
    limit?: number;
    cursor?: string;
  }): Promise<Paginated<DatabaseListItem>> {
    const searchParams: Record<string, unknown> = {
      filter: { property: "object", value: "database" },
      page_size: params?.limit ?? 50,
    };
    if (params?.cursor) searchParams.start_cursor = params.cursor;

    const result = await this.client.search(searchParams as Parameters<typeof this.client.search>[0]);
    return {
      items: result.results.map((r) => transformDatabaseListItem(r as Record<string, unknown>)),
      hasMore: result.has_more,
      nextCursor: result.next_cursor ?? undefined,
    };
  }

  async getDatabase(id: string): Promise<DatabaseDetail> {
    const db = await this.client.databases.retrieve({ database_id: id });
    return transformDatabaseDetail(db as Record<string, unknown>);
  }

  async queryDatabase(params: {
    id: string;
    filter?: unknown;
    sort?: unknown;
    limit?: number;
    cursor?: string;
  }): Promise<Paginated<QueryRow>> {
    const queryParams: Record<string, unknown> = {
      database_id: params.id,
      page_size: params.limit ?? 50,
    };
    if (params.filter) queryParams.filter = params.filter;
    if (params.sort) queryParams.sorts = params.sort;
    if (params.cursor) queryParams.start_cursor = params.cursor;

    const result = await this.client.databases.query(
      queryParams as Parameters<typeof this.client.databases.query>[0],
    );
    return {
      items: result.results.map((r) => transformQueryRow(r as Record<string, unknown>)),
      hasMore: result.has_more,
      nextCursor: result.next_cursor ?? undefined,
    };
  }

  async getDatabaseSchema(id: string): Promise<DatabaseSchema> {
    const db = await this.client.databases.retrieve({ database_id: id });
    return transformDatabaseSchema(db as Record<string, unknown>);
  }

  // --- Pages ---

  async getPage(id: string): Promise<PageDetail> {
    const page = await this.client.pages.retrieve({ page_id: id });
    return transformPageDetail(page as Record<string, unknown>);
  }

  async createPage(params: {
    parentId: string;
    title: string;
    properties?: Record<string, unknown>;
    icon?: string;
  }): Promise<PageCreateResult> {
    const isDb = await this.isDatabase(params.parentId);
    const createParams: Record<string, unknown> = {};

    if (isDb) {
      createParams.parent = { database_id: params.parentId };
      createParams.properties = buildDatabaseProperties(params.title, params.properties);
    } else {
      createParams.parent = { page_id: params.parentId };
      createParams.properties = {
        title: { title: [{ text: { content: params.title } }] },
      };
    }

    if (params.icon) {
      createParams.icon = { type: "emoji", emoji: params.icon };
    }

    const result = await this.client.pages.create(
      createParams as Parameters<typeof this.client.pages.create>[0],
    );
    const p = result as Record<string, unknown>;

    return {
      id: p.id as string,
      url: p.url as string,
      title: params.title,
      parent: p.parent as Record<string, unknown>,
      createdAt: p.created_time as string | undefined,
    };
  }

  async updatePage(params: {
    id: string;
    title?: string;
    properties?: Record<string, unknown>;
    icon?: string;
  }): Promise<PageUpdateResult> {
    const updateParams: Record<string, unknown> = { page_id: params.id };

    if (params.title || params.properties) {
      const props: Record<string, unknown> = {};
      if (params.title) {
        props.title = { title: [{ text: { content: params.title } }] };
      }
      if (params.properties) {
        for (const [key, value] of Object.entries(params.properties)) {
          if (key === "Name" || key === "title") continue;
          props[key] = buildPropertyValue(value);
        }
      }
      updateParams.properties = props;
    }

    if (params.icon) {
      updateParams.icon = { type: "emoji", emoji: params.icon };
    }

    const result = await this.client.pages.update(
      updateParams as Parameters<typeof this.client.pages.update>[0],
    );
    const p = result as Record<string, unknown>;

    return {
      id: p.id as string,
      url: p.url as string,
      lastEditedAt: p.last_edited_time as string | undefined,
    };
  }

  async archivePage(id: string): Promise<PageArchiveResult> {
    await this.client.pages.update({ page_id: id, archived: true } as Parameters<typeof this.client.pages.update>[0]);
    return { id, archived: true };
  }

  // --- Blocks ---

  async listBlocks(params: {
    id: string;
    limit?: number;
    cursor?: string;
  }): Promise<Paginated<NormalizedBlock>> {
    const result = await this.client.blocks.children.list({
      block_id: params.id,
      page_size: params.limit ?? 50,
      start_cursor: params.cursor,
    } as Parameters<typeof this.client.blocks.children.list>[0]);

    return {
      items: result.results.map((b) => normalizeBlock(b as Record<string, unknown>)),
      hasMore: result.has_more,
      nextCursor: result.next_cursor ?? undefined,
    };
  }

  async getAllBlocks(id: string): Promise<BlockListResult> {
    const blocks: NormalizedBlock[] = [];
    let cursor: string | undefined;
    let hasMore = false;

    while (blocks.length < 1000) {
      const response = await this.client.blocks.children.list({
        block_id: id,
        page_size: 100,
        start_cursor: cursor,
      } as Parameters<typeof this.client.blocks.children.list>[0]);

      blocks.push(...response.results.map((b) => normalizeBlock(b as Record<string, unknown>)));

      if (!response.has_more) break;
      if (blocks.length >= 1000) {
        hasMore = true;
        break;
      }
      cursor = response.next_cursor ?? undefined;
    }

    return { blocks, hasMore };
  }

  async getChildBlocks(blockIds: string[]): Promise<Map<string, NormalizedBlock[]>> {
    const childMap = new Map<string, NormalizedBlock[]>();

    // Batch in groups of 5 to avoid rate limits
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
    const result = await this.client.blocks.children.append({
      block_id: params.id,
      children: params.blocks,
    } as Parameters<typeof this.client.blocks.children.append>[0]);
    return { blocksAdded: result.results.length };
  }

  // --- Comments ---

  async listComments(params: {
    pageId: string;
    limit?: number;
    cursor?: string;
  }): Promise<Paginated<CommentItem>> {
    const result = await this.client.comments.list({
      block_id: params.pageId,
      page_size: params.limit ?? 50,
      start_cursor: params.cursor,
    } as Parameters<typeof this.client.comments.list>[0]);

    return {
      items: result.results.map((c) => transformComment(c as Record<string, unknown>)),
      hasMore: result.has_more,
      nextCursor: result.next_cursor ?? undefined,
    };
  }

  async addComment(params: {
    pageId: string;
    body: string;
  }): Promise<CommentCreateResult> {
    const result = await this.client.comments.create({
      parent: { page_id: params.pageId },
      rich_text: [{ type: "text", text: { content: params.body } }],
    } as Parameters<typeof this.client.comments.create>[0]);
    const c = result as Record<string, unknown>;
    const richText = c.rich_text as Array<{ plain_text: string }> | undefined;
    return {
      id: c.id as string,
      body: richText?.map((t) => t.plain_text).join("") ?? params.body,
      createdAt: c.created_time as string | undefined,
    };
  }

  // --- Users ---

  async listUsers(params?: {
    limit?: number;
    cursor?: string;
  }): Promise<Paginated<UserItem>> {
    const result = await this.client.users.list({
      page_size: params?.limit ?? 50,
      start_cursor: params?.cursor,
    } as Parameters<typeof this.client.users.list>[0]);

    return {
      items: result.results.map((u) => transformUser(u as Record<string, unknown>)),
      hasMore: result.has_more,
      nextCursor: result.next_cursor ?? undefined,
    };
  }

  async getMe(): Promise<UserMe> {
    const result = await this.client.users.me({});
    const u = result as Record<string, unknown>;
    const botData = u.bot as { workspace_name?: string } | undefined;
    return {
      id: u.id as string,
      name: u.name as string | undefined,
      type: u.type as "person" | "bot",
      workspaceName: botData?.workspace_name,
    };
  }

  // --- Utility ---

  async isDatabase(id: string): Promise<boolean> {
    try {
      await this.client.databases.retrieve({ database_id: id });
      return true;
    } catch {
      return false;
    }
  }
}

// --- Property value building (for create/update) ---

function buildDatabaseProperties(
  title: string,
  extra?: Record<string, unknown>,
): Record<string, unknown> {
  const props: Record<string, unknown> = {
    Name: { title: [{ text: { content: title } }] },
  };
  if (extra) {
    for (const [key, value] of Object.entries(extra)) {
      if (key === "Name" || key === "title") continue;
      props[key] = buildPropertyValue(value);
    }
  }
  return props;
}

function buildPropertyValue(value: unknown): unknown {
  if (typeof value === "string") return { select: { name: value } };
  if (typeof value === "number") return { number: value };
  if (typeof value === "boolean") return { checkbox: value };
  if (Array.isArray(value)) return { multi_select: value.map((v) => ({ name: String(v) })) };
  return value;
}
