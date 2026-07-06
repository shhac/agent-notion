import { describe, test, expect } from "bun:test";
import type { RecordMap } from "../src/notion/v3/record-map.ts";
import {
  collectDiscussionIds,
  buildAnchorTextMap,
  listComments,
} from "../src/notion/v3/comments.ts";
import { createMockClient } from "./helpers/mock-v3-client.ts";
import { makeBlock, makeComment, makeUser } from "./helpers/fixtures.ts";

function discussionEntry(overrides: Record<string, unknown> & { id: string }) {
  return {
    value: {
      version: 1,
      parent_table: "block",
      resolved: false,
      comments: [],
      ...overrides,
    },
    role: "reader",
  };
}

describe("collectDiscussionIds", () => {
  test("collects discussions from the page and its descendants, deduped", () => {
    const recordMap = {
      block: {
        "page-1": { value: { ...makeBlock({ id: "page-1", content: ["child-1", "child-2"] }), discussions: ["disc-a"] }, role: "reader" },
        "child-1": { value: { ...makeBlock({ id: "child-1", parent_id: "page-1" }), discussions: ["disc-b", "disc-a"] }, role: "reader" },
        "child-2": { value: makeBlock({ id: "child-2", parent_id: "page-1" }), role: "reader" },
      },
    } as RecordMap;

    const ids = collectDiscussionIds(recordMap, "page-1");

    expect(ids.sort()).toEqual(["disc-a", "disc-b"]);
  });

  test("ignores blocks outside the page's content tree", () => {
    const recordMap = {
      block: {
        "page-1": { value: makeBlock({ id: "page-1", content: [] }), role: "reader" },
        "stranger": { value: { ...makeBlock({ id: "stranger" }), discussions: ["disc-x"] }, role: "reader" },
      },
    } as RecordMap;

    expect(collectDiscussionIds(recordMap, "page-1")).toEqual([]);
  });

  test("returns empty for a missing page block", () => {
    expect(collectDiscussionIds({}, "nope")).toEqual([]);
  });
});

describe("buildAnchorTextMap", () => {
  test("extracts anchor text from the discussion's parent block", () => {
    const recordMap = {
      block: {
        "b1": {
          value: makeBlock({
            id: "b1",
            properties: { title: [["Hello "], ["world", [["m", "disc-1"]]]] },
          }),
          role: "reader",
        },
      },
      discussion: {
        "disc-1": discussionEntry({ id: "disc-1", parent_id: "b1" }),
      },
    } as RecordMap;

    const map = buildAnchorTextMap(recordMap, ["disc-1"]);

    expect(map.get("disc-1")).toBe("world");
  });

  test("skips discussions not parented to a block", () => {
    const recordMap = {
      discussion: {
        "disc-1": discussionEntry({ id: "disc-1", parent_id: "page-1", parent_table: "space" }),
      },
    } as RecordMap;

    expect(buildAnchorTextMap(recordMap, ["disc-1"]).size).toBe(0);
  });
});

describe("listComments", () => {
  test("returns transformed comments with author and anchor text", async () => {
    const pageBlock = {
      ...makeBlock({
        id: "page-1",
        content: [],
        properties: { title: [["annotated", [["m", "disc-1"]]]] },
      }),
      discussions: ["disc-1"],
    };
    const { client, calls } = createMockClient({
      loadPageChunk: () => ({
        recordMap: {
          block: { "page-1": { value: pageBlock, role: "reader" } },
          discussion: { "disc-1": discussionEntry({ id: "disc-1", parent_id: "page-1", comments: ["c1"] }) },
          comment: { "c1": { value: makeComment({ id: "c1", parent_id: "disc-1", text: [["Nice!"]] }), role: "reader" } },
          notion_user: { "user-1": { value: makeUser({ id: "user-1" }), role: "reader" } },
        },
        cursor: { stack: [] },
      }),
    });

    const result = await listComments(client, { pageId: "page-1" });

    expect(result.items).toHaveLength(1);
    expect(result.items[0]?.body).toBe("Nice!");
    expect(result.items[0]?.author?.id).toBe("user-1");
    expect(result.items[0]?.anchorText).toBe("annotated");
    // Everything was already in the recordMap — no backfill fetch needed
    expect(calls.syncRecordValues).toBeUndefined();
  });

  test("backfills discussions and comments missing from the recordMap", async () => {
    const pageBlock = { ...makeBlock({ id: "page-1", content: [] }), discussions: ["disc-1"] };
    const { client, calls } = createMockClient({
      loadPageChunk: () => ({
        recordMap: { block: { "page-1": { value: pageBlock, role: "reader" } } },
        cursor: { stack: [] },
      }),
      syncRecordValues: (requests: Array<{ pointer: { table: string } }>) =>
        requests[0]?.pointer.table === "discussion"
          ? { recordMap: { discussion: { "disc-1": discussionEntry({ id: "disc-1", parent_id: "page-1", comments: ["c1"] }) } } }
          : { recordMap: { comment: { "c1": { value: makeComment({ id: "c1", parent_id: "disc-1" }), role: "reader" } } } },
    });

    const result = await listComments(client, { pageId: "page-1" });

    expect(result.items).toHaveLength(1);
    expect(calls.syncRecordValues).toHaveLength(2);
  });

  test("returns empty when the page has no discussions", async () => {
    const { client } = createMockClient({
      loadPageChunk: () => ({
        recordMap: { block: { "page-1": { value: makeBlock({ id: "page-1" }), role: "reader" } } },
        cursor: { stack: [] },
      }),
    });

    const result = await listComments(client, { pageId: "page-1" });

    expect(result.items).toEqual([]);
    expect(result.hasMore).toBe(false);
  });
});
