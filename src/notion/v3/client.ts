/**
 * V3 HTTP client — raw POST requests to notion.so/api/v3/.
 * Uses Bun's built-in fetch. No npm dependencies.
 */
import type { V3Operation } from "./operations.ts";
import type {
  GetAvailableModelsResponse,
  GetInferenceTranscriptsResponse,
  MarkTranscriptSeenResponse,
} from "./ai-types.ts";
import { normalizeRecordMapResponse } from "./record-map.ts";
import type { RecordMap, V3Snapshot, V3Activity } from "./record-map.ts";

const BASE_URL = "https://www.notion.so/api/v3";
const DEFAULT_TIMEOUT = 30_000;
const COLLECTION_TIMEOUT = 60_000;
// Notion may gate features on client version — use a recent desktop version
const NOTION_CLIENT_VERSION = "23.13.20260217.2221";

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

  private buildHeaders(extra?: Record<string, string>): Record<string, string> {
    return {
      "Content-Type": "application/json",
      Cookie: `token_v2=${this.tokenV2}`,
      "x-notion-active-user-header": this.userId,
      "notion-client-version": NOTION_CLIENT_VERSION,
      "notion-audit-log-platform": "web",
      ...extra,
    };
  }

  private static async throwIfNotOk(response: Response, endpoint: string): Promise<void> {
    if (response.ok) return;
    const text = await response.text().catch(() => "");
    throw new V3HttpError(
      `v3 API error: ${response.status} ${response.statusText}${text ? ` — ${text.slice(0, 200)}` : ""}`,
      response.status,
      endpoint,
    );
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
      const response = await fetch(`${BASE_URL}/${endpoint}`, {
        method: "POST",
        headers: this.buildHeaders(options?.extraHeaders),
        body: JSON.stringify(body),
        signal: controller.signal,
      });

      await V3HttpClient.throwIfNotOk(response, endpoint);

      const result = (await response.json()) as T;
      return normalizeRecordMapResponse(result) as T;
    } finally {
      clearTimeout(timer);
    }
  }

  /**
   * POST request that returns the raw Response for streaming consumption.
   * Unlike post<T>(), this does NOT consume response.json().
   * The caller is responsible for reading response.body (e.g., via parseNdjson).
   */
  async postStream(
    endpoint: string,
    body: unknown,
    options?: { timeout?: number; extraHeaders?: Record<string, string> },
  ): Promise<Response> {
    const timeout = options?.timeout ?? DEFAULT_TIMEOUT;
    const controller = new AbortController();
    const timer = setTimeout(() => controller.abort(), timeout);

    try {
      const response = await fetch(`${BASE_URL}/${endpoint}`, {
        method: "POST",
        headers: this.buildHeaders({
          Accept: "application/x-ndjson",
          "x-notion-space-id": this.spaceId,
          ...options?.extraHeaders,
        }),
        body: JSON.stringify(body),
        signal: controller.signal,
      });

      await V3HttpClient.throwIfNotOk(response, endpoint);

      // Don't clear timer here — the caller controls when streaming ends.
      // Instead, clear it when the body is fully consumed or on abort.
      // For safety, set a generous streaming timeout.
      clearTimeout(timer);
      return response;
    } catch (err) {
      clearTimeout(timer);
      throw err;
    }
  }

  // --- Read endpoints ---

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
      page: { id: params.pageId, spaceId: this.spaceId },
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

    const raw = await this.post<{
      result: Record<string, unknown>;
      recordMap: RecordMap;
    }>(
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

    // Normalize: blockIds may be at result.blockIds (old) or
    // result.reducerResults.collection_group_results.blockIds (new)
    const result = raw.result;
    const blockIds: string[] =
      (result.blockIds as string[] | undefined) ??
      ((result.reducerResults as Record<string, unknown>)
        ?.collection_group_results as { blockIds?: string[] })?.blockIds ??
      [];
    const total =
      (result.total as number | undefined) ??
      ((result.reducerResults as Record<string, unknown>)
        ?.collection_group_results as { total?: number })?.total ??
      blockIds.length;

    return {
      result: { blockIds, total, reducerResults: result.reducerResults },
      recordMap: raw.recordMap,
    };
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

  /** Restore a trashed record (page, block, …). Returns the updated recordMap. */
  async restoreRecord(params: {
    id: string;
    table?: string;
  }): Promise<{ recordMap: RecordMap }> {
    return this.post("restoreRecord", {
      pointer: {
        table: params.table ?? "block",
        id: params.id,
        spaceId: this.spaceId,
      },
    });
  }

  get userId_(): string {
    return this.userId;
  }

  get spaceId_(): string {
    return this.spaceId;
  }

  // --- Export endpoints ---

  async enqueueTask(task: {
    eventName: string;
    request: Record<string, unknown>;
  }): Promise<{ taskId: string }> {
    return this.post("enqueueTask", { task });
  }

  async getTasks(
    taskIds: string[],
  ): Promise<{ results: V3ExportTask[] }> {
    return this.post("getTasks", { taskIds });
  }

  // --- Backlinks ---

  async getBacklinksForBlock(params: {
    blockId: string;
  }): Promise<{
    backlinks: Array<{ block_id: string; mentioned_from: { block_id: string; table: string } }>;
    recordMap: RecordMap;
  }> {
    return this.post("getBacklinksForBlock", {
      block: { id: params.blockId, spaceId: this.spaceId },
    });
  }

  // --- Version History ---

  async getSnapshotsList(params: {
    blockId: string;
    size?: number;
  }): Promise<{
    snapshots: V3Snapshot[];
  }> {
    return this.post("getSnapshotsList", {
      block: { id: params.blockId, spaceId: this.spaceId },
      size: params.size ?? 20,
    });
  }

  // --- Activity Log ---

  async getActivityLog(params: {
    navigableBlockId?: string;
    limit?: number;
    startingAfterId?: string;
  }): Promise<{
    activityIds: string[];
    activities: Record<string, V3Activity>;
    recordMap: RecordMap;
  }> {
    return this.post("getActivityLog", {
      spaceId: this.spaceId,
      ...(params.navigableBlockId ? { navigableBlockId: params.navigableBlockId } : {}),
      limit: params.limit ?? 20,
      ...(params.startingAfterId ? { startingAfterId: params.startingAfterId } : {}),
    });
  }

  // --- Record fetching ---

  async syncRecordValuesForPointers(
    pointers: Array<{ id: string; table: string; spaceId?: string }>,
  ): Promise<{ recordMap: RecordMap }> {
    return this.post("syncRecordValuesMain", {
      requests: pointers.map((p) => ({
        pointer: {
          id: p.id,
          table: p.table,
          ...(p.spaceId ? { spaceId: p.spaceId } : {}),
        },
        version: -1,
      })),
    });
  }

  // --- AI endpoints ---

  async getAvailableModels(spaceId: string): Promise<GetAvailableModelsResponse> {
    return this.post("getAvailableModels", { spaceId });
  }

  async getInferenceTranscriptsForUser(params: {
    spaceId: string;
    limit?: number;
  }): Promise<GetInferenceTranscriptsResponse> {
    return this.post("getInferenceTranscriptsForUser", {
      threadParentPointer: {
        table: "space",
        id: params.spaceId,
        spaceId: params.spaceId,
      },
      limit: params.limit ?? 50,
    });
  }

  async markInferenceTranscriptSeen(params: {
    spaceId: string;
    threadId: string;
  }): Promise<MarkTranscriptSeenResponse> {
    return this.post("markInferenceTranscriptSeen", {
      spaceId: params.spaceId,
      threadId: params.threadId,
    });
  }
}

// --- Export task type ---

export type V3ExportTask = {
  id: string;
  eventName: string;
  state: "not_started" | "in_progress" | "success" | "failure";
  status?: {
    type?: string;
    pagesExported?: number;
    exportURL?: string;
  };
  error?: string;
};
