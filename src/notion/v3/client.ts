/**
 * V3 HTTP client — raw POST requests to notion.so/api/v3/.
 * Uses Bun's built-in fetch. No npm dependencies.
 */
import type { V3Operation } from "./operations.ts";

const BASE_URL = "https://www.notion.so/api/v3";
const DEFAULT_TIMEOUT = 30_000;
const COLLECTION_TIMEOUT = 60_000;

export class V3HttpError extends Error {
  constructor(
    message: string,
    public readonly status: number,
    public readonly endpoint: string,
  ) {
    super(message);
    this.name = "V3HttpError";
  }
}

export class V3HttpClient {
  private tokenV2: string;
  private userId: string;
  private spaceId: string;

  constructor(params: { tokenV2: string; userId: string; spaceId: string }) {
    this.tokenV2 = params.tokenV2;
    this.userId = params.userId;
    this.spaceId = params.spaceId;
  }

  private async post<T>(
    endpoint: string,
    body: unknown,
    options?: { timeout?: number; extraHeaders?: Record<string, string> },
  ): Promise<T> {
    const timeout = options?.timeout ?? DEFAULT_TIMEOUT;
    const controller = new AbortController();
    const timer = setTimeout(() => controller.abort(), timeout);

    try {
      const headers: Record<string, string> = {
        "Content-Type": "application/json",
        Cookie: `token_v2=${this.tokenV2}`,
        "x-notion-active-user-header": this.userId,
        ...options?.extraHeaders,
      };

      const response = await fetch(`${BASE_URL}/${endpoint}`, {
        method: "POST",
        headers,
        body: JSON.stringify(body),
        signal: controller.signal,
      });

      if (!response.ok) {
        const text = await response.text().catch(() => "");
        throw new V3HttpError(
          `v3 API error: ${response.status} ${response.statusText}${text ? ` — ${text.slice(0, 200)}` : ""}`,
          response.status,
          endpoint,
        );
      }

      return (await response.json()) as T;
    } finally {
      clearTimeout(timer);
    }
  }

  // --- Read endpoints ---

  async getSpaces(): Promise<Record<string, unknown>> {
    return this.post("getSpaces", {});
  }

  async loadUserContent(): Promise<{ recordMap: RecordMap }> {
    return this.post("loadUserContent", {});
  }

  async loadPageChunk(params: {
    pageId: string;
    limit?: number;
    cursor?: { stack: unknown[] };
    chunkNumber?: number;
  }): Promise<{ recordMap: RecordMap; cursor: { stack: unknown[] } }> {
    return this.post("loadPageChunk", {
      pageId: params.pageId,
      limit: params.limit ?? 100,
      cursor: params.cursor ?? { stack: [] },
      chunkNumber: params.chunkNumber ?? 0,
      verticalColumns: false,
    });
  }

  async syncRecordValues(
    requests: Array<{ pointer: { id: string; table: string }; version: number }>,
  ): Promise<{ recordMap: RecordMap }> {
    return this.post("syncRecordValuesMain", { requests });
  }

  async queryCollection(params: {
    collectionId: string;
    collectionViewId: string;
    query?: { filter?: unknown; sort?: unknown };
    limit?: number;
    searchQuery?: string;
  }): Promise<{
    result: { blockIds: string[]; total: number; reducerResults?: unknown };
    recordMap: RecordMap;
  }> {
    const loader: Record<string, unknown> = {
      type: "reducer",
      reducers: {
        collection_group_results: {
          type: "results",
          limit: params.limit ?? 999,
          loadContentCover: false,
        },
      },
      searchQuery: params.searchQuery ?? "",
      userTimeZone: Intl.DateTimeFormat().resolvedOptions().timeZone,
    };

    const query2: Record<string, unknown> = {};
    if (params.query?.filter) query2.filter = params.query.filter;
    if (params.query?.sort) query2.sort = params.query.sort;

    return this.post(
      "queryCollection",
      {
        collection: { id: params.collectionId },
        collectionView: { id: params.collectionViewId },
        loader,
        query2,
      },
      {
        timeout: COLLECTION_TIMEOUT,
        extraHeaders: { "x-notion-space-id": this.spaceId },
      },
    );
  }

  async search(params: {
    query: string;
    ancestorId?: string;
    limit?: number;
    filters?: Record<string, unknown>;
  }): Promise<{
    results: Array<{ id: string; highlight?: { text: string }; score: number }>;
    total: number;
    recordMap: RecordMap;
  }> {
    return this.post("search", {
      type: params.ancestorId ? "BlocksInAncestor" : "BlocksInSpace",
      ...(params.ancestorId
        ? { ancestorId: params.ancestorId }
        : { spaceId: this.spaceId }),
      query: params.query,
      limit: params.limit ?? 20,
      sort: { field: "relevance" },
      source: "quick_find_input_change",
      filters: {
        isDeletedOnly: false,
        excludeTemplates: false,
        isNavigableOnly: false,
        requireEditPermissions: false,
        ancestors: [],
        createdBy: [],
        editedBy: [],
        lastEditedTime: {},
        createdTime: {},
        ...params.filters,
      },
    });
  }

  async getSignedFileUrls(
    urls: Array<{ url: string; permissionRecord: { table: string; id: string } }>,
  ): Promise<{ signedUrls: string[] }> {
    return this.post("getSignedFileUrls", { urls });
  }

  async submitTransaction(
    operations: Array<{
      id: string;
      table: string;
      path: string[];
      command: string;
      args: unknown;
    }>,
  ): Promise<void> {
    await this.post("submitTransaction", { operations });
  }

  async saveTransactions(operations: V3Operation[]): Promise<void> {
    await this.post("saveTransactions", {
      requestId: crypto.randomUUID(),
      transactions: [
        {
          id: crypto.randomUUID(),
          spaceId: this.spaceId,
          operations,
        },
      ],
    });
  }

  get userId_(): string {
    return this.userId;
  }

  get spaceId_(): string {
    return this.spaceId;
  }
}

// --- RecordMap type ---

export type RecordMap = {
  block?: Record<string, { value: V3Block; role?: string }>;
  collection?: Record<string, { value: V3Collection; role?: string }>;
  collection_view?: Record<string, { value: V3CollectionView; role?: string }>;
  notion_user?: Record<string, { value: V3User; role?: string }>;
  space?: Record<string, { value: V3Space; role?: string }>;
  [table: string]: Record<string, { value: Record<string, unknown>; role?: string }> | undefined;
};

export type V3Block = {
  id: string;
  type: string;
  version: number;
  created_time: number;
  last_edited_time: number;
  parent_id: string;
  parent_table: string;
  alive: boolean;
  properties?: Record<string, V3RichText>;
  content?: string[];
  format?: Record<string, unknown>;
  space_id: string;
  [key: string]: unknown;
};

/** V3 rich text: array of [text, decorations?] tuples */
export type V3RichText = Array<[string] | [string, V3Decoration[]]>;

/** V3 decoration: [type, ...args] */
export type V3Decoration = [string, ...unknown[]];

export type V3Collection = {
  id: string;
  version: number;
  name: V3RichText;
  description?: V3RichText;
  schema: Record<string, V3PropertySchema>;
  parent_id: string;
  parent_table: string;
  icon?: string;
  cover?: string;
  format?: Record<string, unknown>;
  [key: string]: unknown;
};

export type V3PropertySchema = {
  name: string;
  type: string;
  options?: Array<{ id: string; value: string; color?: string }>;
  groups?: Array<{ id: string; name: string; optionIds?: string[]; color?: string }>;
  number_format?: string;
  collection_id?: string;
  [key: string]: unknown;
};

export type V3CollectionView = {
  id: string;
  version: number;
  type: string;
  name?: string;
  parent_id: string;
  parent_table: string;
  alive: boolean;
  format?: Record<string, unknown>;
  query2?: {
    filter?: unknown;
    sort?: unknown;
    aggregations?: unknown;
  };
  [key: string]: unknown;
};

export type V3User = {
  id: string;
  version: number;
  email: string;
  given_name: string;
  family_name: string;
  profile_photo?: string;
  [key: string]: unknown;
};

export type V3Space = {
  id: string;
  version: number;
  name: string;
  icon?: string;
  domain?: string;
  plan_type?: string;
  [key: string]: unknown;
};

export type V3Discussion = {
  id: string;
  version: number;
  parent_id: string;
  parent_table: string;
  resolved: boolean;
  comments: string[];
  [key: string]: unknown;
};

export type V3Comment = {
  id: string;
  version: number;
  alive: boolean;
  parent_id: string;
  parent_table: string;
  text: V3RichText;
  created_by: string;
  created_by_table: string;
  created_time: number;
  last_edited_time: number;
  [key: string]: unknown;
};
