import { describe, test, expect, mock, beforeEach, afterEach } from "bun:test";
import { V3HttpClient } from "../src/notion/v3/client.ts";

// Capture fetch calls to verify request bodies without hitting the network
let fetchCalls: Array<{ url: string; body: unknown }> = [];
const originalFetch = globalThis.fetch;

function installMockFetch(responseBody: unknown, status = 200) {
  fetchCalls = [];
  globalThis.fetch = (async (url: string | URL, opts?: RequestInit) => {
    const body = opts?.body ? JSON.parse(opts.body as string) : undefined;
    fetchCalls.push({ url: String(url), body });
    return new Response(JSON.stringify(responseBody), {
      status,
      headers: { "Content-Type": "application/json" },
    });
  }) as typeof fetch;
}

afterEach(() => {
  globalThis.fetch = originalFetch;
});

function createClient() {
  return new V3HttpClient({
    tokenV2: "fake-token",
    userId: "user-aaa",
    spaceId: "space-bbb",
  });
}

// =============================================================================
// loadPageChunk — request format
// =============================================================================

describe("V3HttpClient.loadPageChunk", () => {
  test("sends page: { id, spaceId } instead of flat pageId", async () => {
    installMockFetch({ recordMap: {}, cursor: { stack: [] } });
    const client = createClient();

    await client.loadPageChunk({ pageId: "page-123", limit: 10 });

    expect(fetchCalls).toHaveLength(1);
    const body = fetchCalls[0]!.body as Record<string, unknown>;
    // New format: page object with id and spaceId
    expect(body.page).toEqual({ id: "page-123", spaceId: "space-bbb" });
    // Old flat pageId should NOT be present
    expect(body.pageId).toBeUndefined();
  });

  test("includes limit, cursor, chunkNumber, and verticalColumns", async () => {
    installMockFetch({ recordMap: {}, cursor: { stack: [] } });
    const client = createClient();

    await client.loadPageChunk({
      pageId: "page-456",
      limit: 50,
      cursor: { stack: [["some-cursor"]] },
      chunkNumber: 2,
    });

    const body = fetchCalls[0]!.body as Record<string, unknown>;
    expect(body.limit).toBe(50);
    expect(body.cursor).toEqual({ stack: [["some-cursor"]] });
    expect(body.chunkNumber).toBe(2);
    expect(body.verticalColumns).toBe(false);
  });
});

// =============================================================================
// queryCollection — response normalization
// =============================================================================

describe("V3HttpClient.queryCollection", () => {
  test("normalizes blockIds from new reducerResults format", async () => {
    // New response format: blockIds nested under reducerResults
    installMockFetch({
      result: {
        type: "reducer",
        reducerResults: {
          collection_group_results: {
            type: "results",
            blockIds: ["row-1", "row-2"],
            hasMore: false,
          },
        },
        sizeHint: 2,
      },
      recordMap: {
        block: {
          "row-1": { value: { id: "row-1", type: "page" }, role: "reader" },
          "row-2": { value: { id: "row-2", type: "page" }, role: "reader" },
        },
      },
    });

    const client = createClient();
    const result = await client.queryCollection({
      collectionId: "col-1",
      collectionViewId: "view-1",
    });

    expect(result.result.blockIds).toEqual(["row-1", "row-2"]);
    expect(result.result.total).toBe(2);
  });

  test("still handles old flat blockIds format", async () => {
    // Old response format: blockIds directly on result
    installMockFetch({
      result: { blockIds: ["row-1"], total: 1 },
      recordMap: {},
    });

    const client = createClient();
    const result = await client.queryCollection({
      collectionId: "col-1",
      collectionViewId: "view-1",
    });

    expect(result.result.blockIds).toEqual(["row-1"]);
    expect(result.result.total).toBe(1);
  });

  test("returns empty blockIds when neither format has data", async () => {
    installMockFetch({
      result: { type: "reducer", reducerResults: {} },
      recordMap: {},
    });

    const client = createClient();
    const result = await client.queryCollection({
      collectionId: "col-1",
      collectionViewId: "view-1",
    });

    expect(result.result.blockIds).toEqual([]);
    expect(result.result.total).toBe(0);
  });
});
