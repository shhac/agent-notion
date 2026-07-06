import { describe, test, expect } from "bun:test";
import type { RecordMap, V3Block, V3Collection } from "../src/notion/v3/record-map.ts";
import {
  normalizeRecordMapResponse,
  unwrapRecordValue,
  mergeRecordMap,
  getBlock,
  getCollection,
  getAllBlocks,
  getFirstCollection,
  getFirstCollectionViewId,
  getFirstUser,
  getFirstSpace,
  getAllUsers,
  getDiscussion,
  getComment,
  getUser,
} from "../src/notion/v3/record-map.ts";
import { makeBlock, makeCollection, makeUser, makeComment } from "./helpers/fixtures.ts";

// =============================================================================
// normalizeRecordMapResponse
// =============================================================================

describe("normalizeRecordMapResponse", () => {
  test("unwraps new spaceId-wrapped recordMap entries", () => {
    const block: V3Block = makeBlock({ id: "page-1" });
    const input = {
      recordMap: {
        block: {
          "page-1": {
            spaceId: "space-1",
            value: { value: block, role: "reader" },
          },
        },
      },
      cursor: { stack: [] },
    };

    const result = normalizeRecordMapResponse(input);

    // Should unwrap to old format: { value: V3Block, role: string }
    const entry = result.recordMap.block["page-1"] as any;
    expect(entry.value).toEqual(block);
    expect(entry.role).toBe("reader");
    expect(entry.spaceId).toBeUndefined();
  });

  test("passes through old-format recordMap entries unchanged", () => {
    const block: V3Block = makeBlock({ id: "page-1" });
    const input = {
      recordMap: {
        block: {
          "page-1": { value: block, role: "reader" },
        },
      },
      cursor: { stack: [] },
    };

    const result = normalizeRecordMapResponse(input);

    const entry = result.recordMap.block["page-1"] as any;
    expect(entry.value).toEqual(block);
    expect(entry.role).toBe("reader");
  });

  test("normalizes all tables in the recordMap", () => {
    const block: V3Block = makeBlock({ id: "page-1" });
    const collection: V3Collection = makeCollection({ id: "col-1", name: [["Test"]] });
    const input = {
      recordMap: {
        block: {
          "page-1": {
            spaceId: "space-1",
            value: { value: block, role: "reader" },
          },
        },
        collection: {
          "col-1": {
            spaceId: "space-1",
            value: { value: collection, role: "reader" },
          },
        },
      },
    };

    const result = normalizeRecordMapResponse(input);

    expect((result.recordMap.block["page-1"] as any).value.id).toBe("page-1");
    expect((result.recordMap.collection["col-1"] as any).value.id).toBe("col-1");
  });

  test("unwraps nested entries without spaceId", () => {
    const block: V3Block = makeBlock({ id: "child-1", type: "paragraph" });
    const input = {
      recordMap: {
        block: {
          "child-1": {
            value: { value: block, role: "reader" },
          },
        },
      },
      cursor: { stack: [] },
    };

    const result = normalizeRecordMapResponse(input);

    const entry = result.recordMap.block["child-1"] as any;
    expect(entry.value).toEqual(block);
    expect(entry.role).toBe("reader");
  });

  test("handles mixed spaceId and non-spaceId entries", () => {
    const parent: V3Block = makeBlock({ id: "page-1", type: "page", content: ["child-1"] });
    const child: V3Block = makeBlock({ id: "child-1", type: "paragraph", parent_id: "page-1" });
    const input = {
      recordMap: {
        block: {
          "page-1": {
            spaceId: "space-1",
            value: { value: parent, role: "reader" },
          },
          "child-1": {
            value: { value: child, role: "reader" },
          },
        },
      },
      cursor: { stack: [] },
    };

    const result = normalizeRecordMapResponse(input);

    expect((result.recordMap.block["page-1"] as any).value.id).toBe("page-1");
    expect((result.recordMap.block["page-1"] as any).value.type).toBe("page");
    expect((result.recordMap.block["child-1"] as any).value.id).toBe("child-1");
    expect((result.recordMap.block["child-1"] as any).value.type).toBe("paragraph");
  });

  test("returns non-recordMap responses unchanged", () => {
    const input = { taskId: "abc-123" };
    const result = normalizeRecordMapResponse(input);
    expect(result).toEqual(input);
  });
});

// =============================================================================
// unwrapRecordValue
// =============================================================================

describe("unwrapRecordValue", () => {
  test("unwraps a role-wrapped value to the entity", () => {
    const entity = { id: "e1", name: "Example" };
    expect(unwrapRecordValue({ value: entity, role: "reader" })).toEqual(entity);
  });

  test("returns a plain entity unchanged", () => {
    const entity = { id: "e1", name: "Example", version: 3 };
    expect(unwrapRecordValue(entity)).toEqual(entity);
  });

  test("returns an entity whose value field is a primitive unchanged", () => {
    const entity = { id: "e1", value: 42 };
    expect(unwrapRecordValue(entity)).toEqual(entity);
  });

  test("returns undefined for null, primitives, and arrays", () => {
    expect(unwrapRecordValue(null)).toBeUndefined();
    expect(unwrapRecordValue(undefined)).toBeUndefined();
    expect(unwrapRecordValue("string")).toBeUndefined();
    expect(unwrapRecordValue(7)).toBeUndefined();
    expect(unwrapRecordValue([{ id: "e1" }])).toBeUndefined();
  });

  test("treats value: null as a plain entity", () => {
    const entity = { id: "e1", value: null };
    expect(unwrapRecordValue(entity)).toEqual(entity);
  });

  // Known limitation of the format sniff: an already-unwrapped entity that
  // legitimately carries an object-valued `value` field is indistinguishable
  // from a role wrapper and gets unwrapped. This is why unwrapRecordValue must
  // only run on data that has NOT passed through normalizeRecordMapResponse
  // (see the RecordMap invariant in record-map.ts).
  test("mis-unwraps an entity with an object-valued value field (documented ambiguity)", () => {
    const entity = { id: "e1", value: { nested: true } };
    expect(unwrapRecordValue(entity)).toEqual({ nested: true });
  });
});

// =============================================================================
// mergeRecordMap
// =============================================================================

describe("mergeRecordMap", () => {
  test("merges records into an existing table", () => {
    const target: RecordMap = {
      block: { "b1": { value: makeBlock({ id: "b1" }), role: "reader" } },
    };
    const source: RecordMap = {
      block: { "b2": { value: makeBlock({ id: "b2" }), role: "reader" } },
    };

    mergeRecordMap(target, source);

    expect(getBlock(target, "b1")?.id).toBe("b1");
    expect(getBlock(target, "b2")?.id).toBe("b2");
  });

  test("creates tables missing from the target", () => {
    const target: RecordMap = {};
    const source: RecordMap = {
      notion_user: { "u1": { value: makeUser({ id: "u1" }), role: "reader" } },
    };

    mergeRecordMap(target, source);

    expect(getUser(target, "u1")?.id).toBe("u1");
  });

  test("source entries overwrite same-id target entries", () => {
    const target: RecordMap = {
      block: { "b1": { value: makeBlock({ id: "b1", type: "text" }), role: "reader" } },
    };
    const source: RecordMap = {
      block: { "b1": { value: makeBlock({ id: "b1", type: "page" }), role: "editor" } },
    };

    mergeRecordMap(target, source);

    expect(getBlock(target, "b1")?.type).toBe("page");
  });
});

// =============================================================================
// RecordMap accessors
// =============================================================================

describe("RecordMap accessors", () => {
  const recordMap: RecordMap = {
    block: {
      "b1": { value: makeBlock({ id: "b1", alive: true }) as any, role: "reader" },
      "b2": { value: makeBlock({ id: "b2", alive: false }) as any, role: "reader" },
      "b3": { value: makeBlock({ id: "b3", alive: true }) as any, role: "reader" },
    },
    collection: {
      "col-1": { value: makeCollection({ id: "col-1" }) as any, role: "reader" },
    },
    collection_view: {
      "view-1": { value: { id: "view-1" } as any, role: "reader" },
    },
    notion_user: {
      "u1": { value: makeUser({ id: "u1" }) as any, role: "reader" },
      "u2": { value: makeUser({ id: "u2", given_name: "Jane" }) as any, role: "reader" },
    },
    space: {
      "s1": { value: { id: "s1", version: 1, name: "Example Workspace" } as any, role: "reader" },
    },
    discussion: {
      "d1": { value: { id: "d1", version: 1, parent_id: "b1", parent_table: "block", resolved: false, comments: ["c1"] } as any, role: "reader" },
    },
    comment: {
      "c1": { value: makeComment({ id: "c1" }) as any, role: "reader" },
    },
  };

  test("getBlock returns block by id", () => {
    expect(getBlock(recordMap, "b1")?.id).toBe("b1");
  });

  test("getBlock returns undefined for missing", () => {
    expect(getBlock(recordMap, "missing")).toBeUndefined();
  });

  test("getCollection returns collection by id", () => {
    expect(getCollection(recordMap, "col-1")?.id).toBe("col-1");
  });

  test("getAllBlocks filters out dead blocks", () => {
    const blocks = getAllBlocks(recordMap);
    expect(blocks).toHaveLength(2);
    expect(blocks.map((b) => b.id)).toEqual(expect.arrayContaining(["b1", "b3"]));
    expect(blocks.find((b) => b.id === "b2")).toBeUndefined();
  });

  test("getAllBlocks returns empty for missing block table", () => {
    expect(getAllBlocks({})).toEqual([]);
  });

  test("getFirstCollection returns first collection", () => {
    expect(getFirstCollection(recordMap)?.id).toBe("col-1");
  });

  test("getFirstCollection returns undefined for empty", () => {
    expect(getFirstCollection({})).toBeUndefined();
  });

  test("getFirstCollectionViewId returns first view id", () => {
    expect(getFirstCollectionViewId(recordMap)).toBe("view-1");
  });

  test("getFirstCollectionViewId returns undefined for empty", () => {
    expect(getFirstCollectionViewId({})).toBeUndefined();
  });

  test("getFirstUser returns first user", () => {
    expect(getFirstUser(recordMap)?.id).toBe("u1");
  });

  test("getFirstUser returns undefined for empty", () => {
    expect(getFirstUser({})).toBeUndefined();
  });

  test("getFirstSpace returns first space", () => {
    expect(getFirstSpace(recordMap)?.name).toBe("Example Workspace");
  });

  test("getFirstSpace returns undefined for empty", () => {
    expect(getFirstSpace({})).toBeUndefined();
  });

  test("getAllUsers returns all users", () => {
    const users = getAllUsers(recordMap);
    expect(users).toHaveLength(2);
  });

  test("getAllUsers returns empty for missing", () => {
    expect(getAllUsers({})).toEqual([]);
  });

  test("getDiscussion returns discussion by id", () => {
    expect(getDiscussion(recordMap, "d1")?.id).toBe("d1");
  });

  test("getComment returns comment by id", () => {
    expect(getComment(recordMap, "c1")?.id).toBe("c1");
  });

  test("getUser returns user by id", () => {
    expect(getUser(recordMap, "u1")?.id).toBe("u1");
  });
});

// =============================================================================
// Role-wrapped wire format end-to-end: normalize then access
// =============================================================================

describe("role-wrapped wire format through the normalizer", () => {
  const wire = (entity: unknown) => ({ value: { value: entity, role: "reader" } });

  test("accessors resolve entities after normalization", () => {
    const input: { recordMap: Record<string, unknown> } = {
      recordMap: {
        block: {
          "b1": wire(makeBlock({ id: "b1" })),
          "b2": wire(makeBlock({ id: "b2", alive: false })),
        },
        notion_user: {
          "u1": wire(makeUser({ id: "u1" })),
        },
      },
    };
    const recordMap = normalizeRecordMapResponse(input).recordMap as RecordMap;

    expect(getBlock(recordMap, "b1")?.id).toBe("b1");
    expect(getUser(recordMap, "u1")?.id).toBe("u1");

    // A dead block nested inside a role wrapper is still filtered out
    const blocks = getAllBlocks(recordMap);
    expect(blocks.map((b) => b.id)).toEqual(["b1"]);
  });
});
