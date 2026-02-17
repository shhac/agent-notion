import { describe, test, expect } from "bun:test";
import type { RecordMap, V3Snapshot, V3Activity } from "../src/notion/v3/client.ts";
import { getBlock, getUser, v3RichTextToPlain } from "../src/notion/v3/transforms.ts";

describe("backlinks response processing", () => {
  const recordMap: RecordMap = {
    block: {
      "block-1": {
        value: {
          id: "block-1",
          type: "text",
          version: 1,
          created_time: 1700000000000,
          last_edited_time: 1700000000000,
          parent_id: "page-a",
          parent_table: "block",
          alive: true,
          properties: { title: [["Some paragraph with a link"]] },
          space_id: "space-1",
        },
        role: "reader",
      },
      "page-a": {
        value: {
          id: "page-a",
          type: "page",
          version: 1,
          created_time: 1700000000000,
          last_edited_time: 1700000000000,
          parent_id: "space-1",
          parent_table: "space",
          alive: true,
          properties: { title: [["Linking Page"]] },
          space_id: "space-1",
        },
        role: "reader",
      },
    },
  };

  test("resolves block to parent page", () => {
    const block = getBlock(recordMap, "block-1");
    expect(block).toBeDefined();
    expect(block!.parent_id).toBe("page-a");

    const parentPage = getBlock(recordMap, block!.parent_id);
    expect(parentPage).toBeDefined();
    expect(v3RichTextToPlain(parentPage!.properties?.title)).toBe("Linking Page");
  });

  test("deduplicates backlinks by pageId", () => {
    const backlinks = [
      { block_id: "target", mentioned_from: { block_id: "block-1", table: "block" } },
      { block_id: "target", mentioned_from: { block_id: "block-2", table: "block" } },
    ];

    // Simulate dedup logic from CLI
    const processed = backlinks.map((bl) => {
      const block = getBlock(recordMap, bl.mentioned_from.block_id);
      const pageBlock = block?.parent_id ? getBlock(recordMap, block.parent_id) : undefined;
      return {
        blockId: bl.mentioned_from.block_id,
        pageId: pageBlock?.id ?? bl.mentioned_from.block_id,
      };
    });

    const seen = new Set<string>();
    const unique = processed.filter((bl) => {
      if (seen.has(bl.pageId)) return false;
      seen.add(bl.pageId);
      return true;
    });

    // block-1 and block-2 both come from page-a, so deduplicated to 1
    // (block-2 isn't in recordMap, so its pageId is itself)
    expect(unique).toHaveLength(2);
    expect(unique[0]!.pageId).toBe("page-a");
    expect(unique[1]!.pageId).toBe("block-2"); // fallback to blockId
  });
});

describe("snapshot processing", () => {
  test("transforms snapshot timestamps to ISO", () => {
    const snapshots: V3Snapshot[] = [
      {
        id: "snap-1",
        version: 10,
        last_version: 15,
        timestamp: 1700000000000,
        authors: [{ id: "user-1", table: "notion_user" }],
      },
      {
        id: "snap-2",
        version: 5,
        last_version: 9,
        timestamp: 1699900000000,
        authors: [{ id: "user-1", table: "notion_user" }, { id: "user-2", table: "notion_user" }],
      },
    ];

    const processed = snapshots.map((snap) => ({
      id: snap.id,
      version: snap.version,
      lastVersion: snap.last_version,
      timestamp: new Date(snap.timestamp).toISOString(),
      authors: snap.authors.map((a) => a.id),
    }));

    expect(processed).toHaveLength(2);
    expect(processed[0]!.id).toBe("snap-1");
    expect(processed[0]!.timestamp).toBe("2023-11-14T22:13:20.000Z");
    expect(processed[0]!.authors).toEqual(["user-1"]);
    expect(processed[1]!.authors).toEqual(["user-1", "user-2"]);
  });
});

describe("activity log processing", () => {
  const recordMap: RecordMap = {
    block: {
      "page-1": {
        value: {
          id: "page-1",
          type: "page",
          version: 1,
          created_time: 1700000000000,
          last_edited_time: 1700000000000,
          parent_id: "space-1",
          parent_table: "space",
          alive: true,
          properties: { title: [["My Page"]] },
          space_id: "space-1",
        },
        role: "reader",
      },
    },
    notion_user: {
      "user-1": {
        value: {
          id: "user-1",
          version: 1,
          email: "test@example.com",
          given_name: "Jane",
          family_name: "Doe",
        },
        role: "reader",
      },
    },
  };

  const activities: Record<string, V3Activity> = {
    "act-1": {
      id: "act-1",
      version: 1,
      type: "block-edited",
      parent_id: "page-1",
      parent_table: "block",
      navigable_block_id: "page-1",
      space_id: "space-1",
      edits: [
        {
          type: "block-changed",
          block_id: "page-1",
          timestamp: 1700000000000,
          authors: [{ id: "user-1", table: "notion_user" }],
        },
      ],
      start_time: 1700000000000,
      end_time: 1700001000000,
    },
  };

  test("resolves page titles from recordMap", () => {
    const activity = activities["act-1"]!;
    const blockId = activity.navigable_block_id ?? activity.parent_id;
    const block = getBlock(recordMap, blockId);

    expect(block).toBeDefined();
    expect(v3RichTextToPlain(block!.properties?.title)).toBe("My Page");
  });

  test("resolves author names from recordMap", () => {
    const activity = activities["act-1"]!;
    const authors = (activity.edits ?? [])
      .flatMap((e) => e.authors ?? [])
      .map((a) => {
        const user = getUser(recordMap, a.id);
        return user
          ? [user.given_name, user.family_name].filter(Boolean).join(" ")
          : a.id;
      });

    expect(authors).toEqual(["Jane Doe"]);
  });

  test("deduplicates authors", () => {
    const activityWithDupes: V3Activity = {
      ...activities["act-1"]!,
      edits: [
        {
          type: "block-changed",
          timestamp: 1700000000000,
          authors: [{ id: "user-1", table: "notion_user" }],
        },
        {
          type: "block-changed",
          timestamp: 1700000001000,
          authors: [{ id: "user-1", table: "notion_user" }],
        },
      ],
    };

    const authors = (activityWithDupes.edits ?? [])
      .flatMap((e) => e.authors ?? [])
      .map((a) => {
        const user = getUser(recordMap, a.id);
        return user
          ? [user.given_name, user.family_name].filter(Boolean).join(" ")
          : a.id;
      });
    const unique = [...new Set(authors)];

    expect(unique).toEqual(["Jane Doe"]);
  });

  test("handles timestamps", () => {
    const activity = activities["act-1"]!;
    expect(new Date(activity.start_time!).toISOString()).toBe("2023-11-14T22:13:20.000Z");
    expect(new Date(activity.end_time!).toISOString()).toBe("2023-11-14T22:30:00.000Z");
  });
});
