import { describe, test, expect } from "bun:test";
import type { RecordMap, V3Block, V3Collection, V3User, V3Comment, V3Discussion, V3ExportTask } from "../src/notion/v3/client.ts";
import { V3Backend } from "../src/notion/v3/backend.ts";
import type { V3Operation } from "../src/notion/v3/operations.ts";

// --- Mock HTTP Client ---

type MockResponses = {
  search?: () => any;
  loadPageChunk?: (params: any) => any;
  syncRecordValues?: (requests: any) => any;
  queryCollection?: (params: any) => any;
  saveTransactions?: (ops: V3Operation[]) => void;
  loadUserContent?: () => any;
  enqueueTask?: (task: any) => any;
  getTasks?: (taskIds: string[]) => any;
};

function createMockClient(responses: MockResponses = {}) {
  const calls: Record<string, any[]> = {};

  function track(method: string, args: any) {
    if (!calls[method]) calls[method] = [];
    calls[method].push(args);
  }

  const client = {
    get spaceId_() { return "space-1"; },
    get userId_() { return "user-1"; },
    search: async (params: any) => {
      track("search", params);
      return responses.search?.() ?? { results: [], total: 0, recordMap: {} };
    },
    loadPageChunk: async (params: any) => {
      track("loadPageChunk", params);
      return responses.loadPageChunk?.(params) ?? { recordMap: {}, cursor: { stack: [] } };
    },
    syncRecordValues: async (requests: any) => {
      track("syncRecordValues", requests);
      return responses.syncRecordValues?.(requests) ?? { recordMap: {} };
    },
    queryCollection: async (params: any) => {
      track("queryCollection", params);
      return responses.queryCollection?.(params) ?? { result: { blockIds: [], total: 0 }, recordMap: {} };
    },
    saveTransactions: async (ops: V3Operation[]) => {
      track("saveTransactions", ops);
      responses.saveTransactions?.(ops);
    },
    loadUserContent: async () => {
      track("loadUserContent", {});
      return responses.loadUserContent?.() ?? { recordMap: {} };
    },
  };

  return { client: client as any, calls };
}

// --- Helpers ---

function makeBlock(overrides: Partial<V3Block> & { id: string }): V3Block {
  return {
    type: "page",
    version: 1,
    created_time: 1700000000000,
    last_edited_time: 1700001000000,
    parent_id: "parent-1",
    parent_table: "block",
    alive: true,
    space_id: "space-1",
    ...overrides,
  };
}

function wrapBlock(block: V3Block): { value: V3Block; role: string } {
  return { value: block, role: "reader" };
}

// =============================================================================
// search
// =============================================================================

describe("V3Backend.search", () => {
  test("returns transformed search results", async () => {
    const block = makeBlock({ id: "page-1", type: "page", properties: { title: [["My Page"]] } });
    const { client } = createMockClient({
      search: () => ({
        results: [{ id: "page-1", score: 1 }],
        total: 1,
        recordMap: { block: { "page-1": wrapBlock(block) } },
      }),
    });

    const backend = new V3Backend(client);
    const result = await backend.search({ query: "test" });

    expect(result.items).toHaveLength(1);
    expect(result.items[0]!.id).toBe("page-1");
    expect(result.items[0]!.title).toBe("My Page");
    expect(result.items[0]!.type).toBe("page");
  });

  test("filters out databases when filter=page", async () => {
    const pageBlock = makeBlock({ id: "p1", type: "page", properties: { title: [["Page"]] } });
    const dbBlock = makeBlock({ id: "d1", type: "collection_view_page", properties: { title: [["DB"]] } });
    const { client } = createMockClient({
      search: () => ({
        results: [{ id: "p1", score: 1 }, { id: "d1", score: 1 }],
        total: 2,
        recordMap: { block: { "p1": wrapBlock(pageBlock), "d1": wrapBlock(dbBlock) } },
      }),
    });

    const backend = new V3Backend(client);
    const result = await backend.search({ query: "test", filter: "page" });

    expect(result.items).toHaveLength(1);
    expect(result.items[0]!.type).toBe("page");
  });

  test("filters out pages when filter=database", async () => {
    const pageBlock = makeBlock({ id: "p1", type: "page", properties: { title: [["Page"]] } });
    const dbBlock = makeBlock({ id: "d1", type: "collection_view_page", properties: { title: [["DB"]] } });
    const { client } = createMockClient({
      search: () => ({
        results: [{ id: "p1", score: 1 }, { id: "d1", score: 1 }],
        total: 2,
        recordMap: { block: { "p1": wrapBlock(pageBlock), "d1": wrapBlock(dbBlock) } },
      }),
    });

    const backend = new V3Backend(client);
    const result = await backend.search({ query: "test", filter: "database" });

    expect(result.items).toHaveLength(1);
    expect(result.items[0]!.type).toBe("database");
  });

  test("skips blocks not found in recordMap", async () => {
    const { client } = createMockClient({
      search: () => ({
        results: [{ id: "missing-block", score: 1 }],
        total: 1,
        recordMap: { block: {} },
      }),
    });

    const backend = new V3Backend(client);
    const result = await backend.search({ query: "test" });
    expect(result.items).toHaveLength(0);
  });
});

// =============================================================================
// getPage
// =============================================================================

describe("V3Backend.getPage", () => {
  test("returns page detail for standalone page", async () => {
    const block = makeBlock({
      id: "page-1",
      type: "page",
      properties: { title: [["Hello"]] },
      parent_table: "block",
      parent_id: "parent-1",
      format: { page_icon: "📝" },
    });
    const { client } = createMockClient({
      loadPageChunk: () => ({
        recordMap: { block: { "page-1": wrapBlock(block) } },
        cursor: { stack: [] },
      }),
    });

    const backend = new V3Backend(client);
    const result = await backend.getPage("page-1");

    expect(result.id).toBe("page-1");
    expect(result.properties).toEqual({ title: "Hello" });
    expect(result.icon).toEqual({ type: "emoji", emoji: "📝" });
    expect(result.parent).toEqual({ type: "page", id: "parent-1" });
  });

  test("resolves schema for database row page", async () => {
    const block = makeBlock({
      id: "row-1",
      parent_table: "collection",
      parent_id: "col-1",
      properties: { title: [["Row 1"]], abc1: [["Done"]] },
    });
    const collection: V3Collection = {
      id: "col-1",
      version: 1,
      name: [["My DB"]],
      schema: {
        title: { name: "Name", type: "title" },
        abc1: { name: "Status", type: "status" },
      },
      parent_id: "parent-1",
      parent_table: "block",
    };
    const { client } = createMockClient({
      loadPageChunk: () => ({
        recordMap: {
          block: { "row-1": wrapBlock(block) },
          collection: { "col-1": { value: collection, role: "reader" } },
        },
        cursor: { stack: [] },
      }),
    });

    const backend = new V3Backend(client);
    const result = await backend.getPage("row-1");

    expect(result.properties).toEqual({ Name: "Row 1", Status: "Done" });
    expect(result.parent).toEqual({ type: "database", id: "col-1" });
  });

  test("throws when page not found", async () => {
    const { client } = createMockClient({
      loadPageChunk: () => ({
        recordMap: { block: {} },
        cursor: { stack: [] },
      }),
    });

    const backend = new V3Backend(client);
    await expect(backend.getPage("missing")).rejects.toThrow(/not found/);
  });
});

// =============================================================================
// listBlocks
// =============================================================================

describe("V3Backend.listBlocks", () => {
  test("returns normalized child blocks", async () => {
    const parent = makeBlock({ id: "page-1", content: ["b1", "b2"] });
    const child1 = makeBlock({ id: "b1", type: "text", properties: { title: [["Hello"]] }, alive: true });
    const child2 = makeBlock({ id: "b2", type: "header", properties: { title: [["Heading"]] }, alive: true });
    const { client } = createMockClient({
      loadPageChunk: () => ({
        recordMap: {
          block: {
            "page-1": wrapBlock(parent),
            "b1": wrapBlock(child1),
            "b2": wrapBlock(child2),
          },
        },
        cursor: { stack: [] },
      }),
    });

    const backend = new V3Backend(client);
    const result = await backend.listBlocks({ id: "page-1" });

    expect(result.items).toHaveLength(2);
    expect(result.items[0]!.type).toBe("paragraph");
    expect(result.items[0]!.richText).toBe("Hello");
    expect(result.items[1]!.type).toBe("heading_1");
  });

  test("skips dead blocks", async () => {
    const parent = makeBlock({ id: "page-1", content: ["b1", "b2"] });
    const alive = makeBlock({ id: "b1", type: "text", alive: true });
    const dead = makeBlock({ id: "b2", type: "text", alive: false });
    const { client } = createMockClient({
      loadPageChunk: () => ({
        recordMap: {
          block: {
            "page-1": wrapBlock(parent),
            "b1": wrapBlock(alive),
            "b2": wrapBlock(dead),
          },
        },
        cursor: { stack: [] },
      }),
    });

    const backend = new V3Backend(client);
    const result = await backend.listBlocks({ id: "page-1" });
    expect(result.items).toHaveLength(1);
  });
});

// =============================================================================
// archivePage
// =============================================================================

describe("V3Backend.archivePage", () => {
  test("sends archive operations", async () => {
    const block = makeBlock({ id: "page-1", parent_id: "parent-1", parent_table: "block" });
    const { client, calls } = createMockClient({
      loadPageChunk: () => ({
        recordMap: { block: { "page-1": wrapBlock(block) } },
        cursor: { stack: [] },
      }),
    });

    const backend = new V3Backend(client);
    const result = await backend.archivePage("page-1");

    expect(result).toEqual({ id: "page-1", archived: true });
    expect(calls.saveTransactions).toHaveLength(1);
    const ops = calls.saveTransactions![0] as V3Operation[];
    expect(ops).toHaveLength(3);
    // update alive: false
    expect(ops[0]!.command).toBe("update");
    expect((ops[0]!.args as any).alive).toBe(false);
    // listRemove
    expect(ops[1]!.command).toBe("listRemove");
    // editMeta
    expect(ops[2]!.command).toBe("update");
  });

  test("throws when page not found", async () => {
    const { client } = createMockClient({
      loadPageChunk: () => ({
        recordMap: { block: {} },
        cursor: { stack: [] },
      }),
    });

    const backend = new V3Backend(client);
    await expect(backend.archivePage("missing")).rejects.toThrow(/not found/);
  });
});

// =============================================================================
// listComments
// =============================================================================

describe("V3Backend.listComments", () => {
  test("returns comments with resolved authors", async () => {
    const block = makeBlock({
      id: "page-1",
      discussions: ["disc-1"],
    } as any);
    const discussion: V3Discussion = {
      id: "disc-1",
      version: 1,
      parent_id: "page-1",
      parent_table: "block",
      resolved: false,
      comments: ["c1"],
    };
    const comment: V3Comment = {
      id: "c1",
      version: 1,
      alive: true,
      parent_id: "disc-1",
      parent_table: "discussion",
      text: [["Nice work"]],
      created_by_id: "u1",
      created_by_table: "notion_user",
      created_time: 1700000000000,
      last_edited_time: 1700000000000,
    };
    const user: V3User = {
      id: "u1",
      version: 1,
      email: "jane@example.com",
      given_name: "Jane",
      family_name: "Doe",
    };

    const { client } = createMockClient({
      loadPageChunk: () => ({
        recordMap: {
          block: { "page-1": { value: block, role: "reader" } },
          discussion: { "disc-1": { value: discussion, role: "reader" } },
          comment: { "c1": { value: comment, role: "reader" } },
          notion_user: { "u1": { value: user, role: "reader" } },
        },
        cursor: { stack: [] },
      }),
    });

    const backend = new V3Backend(client);
    const result = await backend.listComments({ pageId: "page-1" });

    expect(result.items).toHaveLength(1);
    expect(result.items[0]!.body).toBe("Nice work");
    expect(result.items[0]!.author).toEqual({ id: "u1", name: "Jane Doe" });
  });

  test("returns empty when no discussions", async () => {
    const block = makeBlock({ id: "page-1" });
    const { client } = createMockClient({
      loadPageChunk: () => ({
        recordMap: { block: { "page-1": wrapBlock(block) } },
        cursor: { stack: [] },
      }),
    });

    const backend = new V3Backend(client);
    const result = await backend.listComments({ pageId: "page-1" });
    expect(result.items).toHaveLength(0);
  });

  test("fetches missing discussions and comments via syncRecordValues", async () => {
    const block = makeBlock({
      id: "page-1",
      discussions: ["disc-1"],
    } as any);

    const discussion: V3Discussion = {
      id: "disc-1",
      version: 1,
      parent_id: "page-1",
      parent_table: "block",
      resolved: false,
      comments: ["c1"],
    };
    const comment: V3Comment = {
      id: "c1",
      version: 1,
      alive: true,
      parent_id: "disc-1",
      parent_table: "discussion",
      text: [["Hello"]],
      created_by_id: "u1",
      created_by_table: "notion_user",
      created_time: 1700000000000,
      last_edited_time: 1700000000000,
    };

    let syncCallCount = 0;
    const { client, calls } = createMockClient({
      loadPageChunk: () => ({
        recordMap: {
          block: { "page-1": { value: block, role: "reader" } },
          // discussion and comment NOT included — forces sync
        },
        cursor: { stack: [] },
      }),
      syncRecordValues: (requests: any) => {
        syncCallCount++;
        if (syncCallCount === 1) {
          // First call: fetch discussion
          return { recordMap: { discussion: { "disc-1": { value: discussion, role: "reader" } } } };
        }
        // Second call: fetch comment
        return { recordMap: { comment: { "c1": { value: comment, role: "reader" } } } };
      },
    });

    const backend = new V3Backend(client);
    const result = await backend.listComments({ pageId: "page-1" });

    expect(result.items).toHaveLength(1);
    expect(result.items[0]!.body).toBe("Hello");
    expect(calls.syncRecordValues).toHaveLength(2);
  });
});

// =============================================================================
// addComment
// =============================================================================

describe("V3Backend.addComment", () => {
  test("sends comment operations and returns result", async () => {
    const { client, calls } = createMockClient();

    const backend = new V3Backend(client);
    const result = await backend.addComment({ pageId: "page-1", body: "Test comment" });

    expect(result.body).toBe("Test comment");
    expect(result.id).toBeDefined();
    expect(result.discussionId).toBeDefined();
    expect(result.createdAt).toBeDefined();

    // Verify saveTransactions was called with 6 ops (createCommentOps)
    expect(calls.saveTransactions).toHaveLength(1);
    const ops = calls.saveTransactions![0] as V3Operation[];
    expect(ops).toHaveLength(6);
  });
});

// =============================================================================
// listUsers / getMe
// =============================================================================

describe("V3Backend.listUsers", () => {
  test("returns transformed users", async () => {
    const user1: V3User = { id: "u1", version: 1, email: "a@b.com", given_name: "Alice", family_name: "B" };
    const user2: V3User = { id: "u2", version: 1, email: "c@d.com", given_name: "Charlie", family_name: "D" };

    const { client } = createMockClient({
      loadUserContent: () => ({
        recordMap: {
          notion_user: {
            "u1": { value: user1, role: "reader" },
            "u2": { value: user2, role: "reader" },
          },
        },
      }),
    });

    const backend = new V3Backend(client);
    const result = await backend.listUsers();

    expect(result.items).toHaveLength(2);
    expect(result.items[0]!.name).toBe("Alice B");
    expect(result.items[1]!.name).toBe("Charlie D");
    expect(result.hasMore).toBe(false);
  });
});

describe("V3Backend.getMe", () => {
  test("returns current user with workspace name", async () => {
    const user: V3User = { id: "u1", version: 1, email: "a@b.com", given_name: "Alice", family_name: "B" };

    const { client } = createMockClient({
      loadUserContent: () => ({
        recordMap: {
          notion_user: { "u1": { value: user, role: "reader" } },
          space: { "s1": { value: { id: "s1", name: "My Workspace" }, role: "reader" } },
        },
      }),
    });

    const backend = new V3Backend(client);
    const result = await backend.getMe();

    expect(result.id).toBe("u1");
    expect(result.name).toBe("Alice B");
    expect(result.workspaceName).toBe("My Workspace");
  });

  test("throws when no user found", async () => {
    const { client } = createMockClient({
      loadUserContent: () => ({
        recordMap: { notion_user: {} },
      }),
    });

    const backend = new V3Backend(client);
    await expect(backend.getMe()).rejects.toThrow(/user information/);
  });
});

// =============================================================================
// isDatabase
// =============================================================================

describe("V3Backend.isDatabase", () => {
  test("returns true for collection_view_page", async () => {
    const block = makeBlock({ id: "db-1", type: "collection_view_page" });
    const { client } = createMockClient({
      loadPageChunk: () => ({
        recordMap: { block: { "db-1": wrapBlock(block) } },
        cursor: { stack: [] },
      }),
    });

    const backend = new V3Backend(client);
    expect(await backend.isDatabase("db-1")).toBe(true);
  });

  test("returns true for collection_view", async () => {
    const block = makeBlock({ id: "db-1", type: "collection_view" });
    const { client } = createMockClient({
      loadPageChunk: () => ({
        recordMap: { block: { "db-1": wrapBlock(block) } },
        cursor: { stack: [] },
      }),
    });

    const backend = new V3Backend(client);
    expect(await backend.isDatabase("db-1")).toBe(true);
  });

  test("returns false for page", async () => {
    const block = makeBlock({ id: "p1", type: "page" });
    const { client } = createMockClient({
      loadPageChunk: () => ({
        recordMap: { block: { "p1": wrapBlock(block) } },
        cursor: { stack: [] },
      }),
    });

    const backend = new V3Backend(client);
    expect(await backend.isDatabase("p1")).toBe(false);
  });

  test("returns false on error", async () => {
    const { client } = createMockClient({
      loadPageChunk: () => { throw new Error("Not found"); },
    });

    const backend = new V3Backend(client);
    expect(await backend.isDatabase("missing")).toBe(false);
  });
});

// =============================================================================
// getDatabase
// =============================================================================

describe("V3Backend.getDatabase", () => {
  test("returns database detail with resolved collection", async () => {
    const block = makeBlock({
      id: "db-1",
      type: "collection_view_page",
      collection_id: "col-1",
    } as any);
    const collection: V3Collection = {
      id: "col-1",
      version: 1,
      name: [["Tasks"]],
      schema: {
        title: { name: "Name", type: "title" },
        abc1: { name: "Status", type: "select" },
      },
      parent_id: "parent-1",
      parent_table: "block",
    };

    const { client } = createMockClient({
      loadPageChunk: () => ({
        recordMap: {
          block: { "db-1": wrapBlock(block) },
          collection: { "col-1": { value: collection, role: "reader" } },
        },
        cursor: { stack: [] },
      }),
    });

    const backend = new V3Backend(client);
    const result = await backend.getDatabase("db-1");

    expect(result.id).toBe("db-1");
    expect(result.title).toBe("Tasks");
    expect(Object.keys(result.properties)).toEqual(["Name", "Status"]);
  });

  test("throws when database not found", async () => {
    const block = makeBlock({ id: "page-1", type: "page" });
    const { client } = createMockClient({
      loadPageChunk: () => ({
        recordMap: { block: { "page-1": wrapBlock(block) } },
        cursor: { stack: [] },
      }),
    });

    const backend = new V3Backend(client);
    await expect(backend.getDatabase("page-1")).rejects.toThrow(/not found/);
  });
});

// =============================================================================
// getAllBlocks (pagination)
// =============================================================================

describe("V3Backend.getAllBlocks", () => {
  test("collects blocks across chunks", async () => {
    let callCount = 0;
    const { client } = createMockClient({
      loadPageChunk: (params: any) => {
        callCount++;
        if (callCount === 1) {
          return {
            recordMap: {
              block: {
                "page-1": wrapBlock(makeBlock({ id: "page-1", content: ["b1", "b2"] })),
                "b1": wrapBlock(makeBlock({ id: "b1", type: "text", properties: { title: [["Block 1"]] } })),
                "b2": wrapBlock(makeBlock({ id: "b2", type: "text", properties: { title: [["Block 2"]] } })),
              },
            },
            cursor: { stack: [["something"]] }, // has more
          };
        }
        // Second chunk — no more
        return {
          recordMap: {
            block: {
              "page-1": wrapBlock(makeBlock({ id: "page-1", content: ["b1", "b2", "b3"] })),
              "b3": wrapBlock(makeBlock({ id: "b3", type: "text", properties: { title: [["Block 3"]] } })),
            },
          },
          cursor: { stack: [] },
        };
      },
    });

    const backend = new V3Backend(client);
    const result = await backend.getAllBlocks("page-1");

    expect(result.blocks.length).toBe(3);
    expect(result.hasMore).toBe(false);
  });

  test("deduplicates blocks across chunks", async () => {
    let callCount = 0;
    const { client } = createMockClient({
      loadPageChunk: () => {
        callCount++;
        if (callCount === 1) {
          return {
            recordMap: {
              block: {
                "page-1": wrapBlock(makeBlock({ id: "page-1", content: ["b1"] })),
                "b1": wrapBlock(makeBlock({ id: "b1", type: "text" })),
              },
            },
            cursor: { stack: [["more"]] },
          };
        }
        // Second chunk includes same block
        return {
          recordMap: {
            block: {
              "page-1": wrapBlock(makeBlock({ id: "page-1", content: ["b1"] })),
              "b1": wrapBlock(makeBlock({ id: "b1", type: "text" })),
            },
          },
          cursor: { stack: [] },
        };
      },
    });

    const backend = new V3Backend(client);
    const result = await backend.getAllBlocks("page-1");
    expect(result.blocks).toHaveLength(1);
  });
});
