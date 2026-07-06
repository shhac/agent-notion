/**
 * Live integration tests — run against a real Notion page.
 *
 * Requires NOTION_TEST_PAGE_URL env var pointing to a writable page.
 * Skipped entirely when the env var is not set.
 *
 * Usage:
 *   NOTION_TEST_PAGE_URL="https://www.notion.so/workspace/Page-Name-abc123def456" bun test test/integration.test.ts
 */
import { describe, expect, test, beforeAll, afterAll } from "bun:test";
import { execSync } from "node:child_process";

const TEST_PAGE_URL = process.env["NOTION_TEST_PAGE_URL"];

function extractPageId(url: string): string {
  // Notion URLs end with a 32-char hex ID (with or without dashes, sometimes after a title slug)
  const match = url.match(/([a-f0-9]{32})$/i) ?? url.match(/([a-f0-9-]{36})$/);
  if (!match) throw new Error(`Could not extract page ID from URL: ${url}`);
  return match[1]!;
}

function run(args: string): string {
  return execSync(`bun run src/index.ts ${args}`, {
    encoding: "utf8",
    timeout: 30000,
  }).trim();
}

function runJson(args: string): Record<string, unknown> {
  return JSON.parse(run(args));
}

describe.skipIf(!TEST_PAGE_URL)("integration tests", () => {
  const pageId = TEST_PAGE_URL ? extractPageId(TEST_PAGE_URL) : "";

  // --- Setup: clear the page before tests ---

  beforeAll(() => {
    runJson(`block replace ${pageId} --content "Integration test setup"`);
  });

  // --- Teardown: restore clean state ---

  afterAll(() => {
    try {
      runJson(`block replace ${pageId} --content "Integration tests completed."`);
    } catch {
      // best-effort cleanup
    }
  });

  // --- Block list ---

  describe("block list", () => {
    test("returns markdown content", () => {
      const result = runJson(`block list ${pageId}`);
      expect(result.pageId).toContain(pageId.replace(/-/g, "").slice(0, 8));
      expect(result.content).toBe("Integration test setup");
      expect(result.blockCount).toBe(1);
    });

    test("returns raw block objects", () => {
      const result = runJson(`block list ${pageId} --raw`);
      const items = result.items as Array<Record<string, unknown>>;
      expect(items.length).toBe(1);
      expect(items[0]!.type).toBe("paragraph");
      expect(items[0]!.content).toBe("Integration test setup");
      expect(items[0]!.id).toBeDefined();
    });
  });

  // --- Block append ---

  describe("block append", () => {
    test("appends markdown content", () => {
      const result = runJson(`block append ${pageId} --content "## Test Heading\n\nTest paragraph"`);
      expect(result.blocksAdded).toBe(2);
    });

    test("appends JSON blocks", () => {
      const blocks = JSON.stringify([
        { type: "paragraph", paragraph: { rich_text: [{ type: "text", text: { content: "JSON block" } }] } },
      ]);
      const result = runJson(`block append ${pageId} --blocks '${blocks}'`);
      expect(result.blocksAdded).toBe(1);
    });

    test("appended content is readable", () => {
      const result = runJson(`block list ${pageId}`);
      const content = result.content as string;
      expect(content).toContain("Test Heading");
      expect(content).toContain("Test paragraph");
      expect(content).toContain("JSON block");
    });
  });

  // --- Block update ---

  describe("block update", () => {
    test("updates a block's content", () => {
      // Get the first block ID
      const listResult = runJson(`block list ${pageId} --raw`);
      const items = listResult.items as Array<Record<string, unknown>>;
      const blockId = items[0]!.id as string;

      const result = runJson(`block update ${blockId} --content "Updated content"`);
      expect(result.id).toBe(blockId);
      expect(result.lastEditedAt).toBeDefined();

      // Verify the update
      const after = runJson(`block list ${pageId} --raw`);
      const updated = (after.items as Array<Record<string, unknown>>)[0]!;
      expect(updated.content).toBe("Updated content");
    });
  });

  // --- Block delete ---

  describe("block delete", () => {
    test("deletes a block", () => {
      // Get block count before
      const before = runJson(`block list ${pageId} --raw`);
      const beforeItems = before.items as Array<Record<string, unknown>>;
      const countBefore = beforeItems.length;

      // Delete the last block
      const lastBlockId = beforeItems[countBefore - 1]!.id as string;
      const result = runJson(`block delete ${lastBlockId}`);
      expect(result.id).toBe(lastBlockId);
      expect(result.deleted).toBe(true);

      // Verify block count decreased
      const after = runJson(`block list ${pageId} --raw`);
      const afterItems = after.items as Array<Record<string, unknown>>;
      expect(afterItems.length).toBe(countBefore - 1);
    });
  });

  // --- Block replace ---

  describe("block replace", () => {
    test("replaces all content", () => {
      const result = runJson(`block replace ${pageId} --content "# Fresh Start\n\nClean slate."`);
      expect(result.blocksDeleted).toBeGreaterThan(0);
      expect(result.blocksAdded).toBe(2);

      const after = runJson(`block list ${pageId}`);
      expect(after.content).toContain("Fresh Start");
      expect(after.content).toContain("Clean slate.");
      expect(after.blockCount).toBe(2);
    });
  });

  // --- Block move ---

  describe("block move", () => {
    test("reorders blocks within a page", () => {
      // Set up three blocks
      runJson(`block replace ${pageId} --content "Block A\nBlock B\nBlock C"`);

      const listResult = runJson(`block list ${pageId} --raw`);
      const items = listResult.items as Array<Record<string, unknown>>;
      expect(items.length).toBe(3);
      const [blockA, _blockB, blockC] = items;

      // Move C to first position
      const moveResult = runJson(`block move ${blockC!.id}`);
      expect(moveResult.id).toBe(blockC!.id);

      // Verify new order: C, A, B
      const after = runJson(`block list ${pageId} --raw`);
      const afterItems = after.items as Array<Record<string, unknown>>;
      expect(afterItems[0]!.content).toBe("Block C");
      expect(afterItems[1]!.content).toBe("Block A");
      expect(afterItems[2]!.content).toBe("Block B");
    });

    test("moves block after a specific block", () => {
      const listResult = runJson(`block list ${pageId} --raw`);
      const items = listResult.items as Array<Record<string, unknown>>;
      // Current order: C, A, B — move A after B
      const [blockC, blockA, blockB] = items;

      runJson(`block move ${blockA!.id} --after ${blockB!.id}`);

      const after = runJson(`block list ${pageId} --raw`);
      const afterItems = after.items as Array<Record<string, unknown>>;
      expect(afterItems[0]!.content).toBe("Block C");
      expect(afterItems[1]!.content).toBe("Block B");
      expect(afterItems[2]!.content).toBe("Block A");
    });

    test("moves block into a container", () => {
      // Set up a callout + a paragraph
      runJson(`block replace ${pageId} --content "Standalone text"`);
      const calloutBlocks = JSON.stringify([
        { type: "callout", callout: { rich_text: [{ type: "text", text: { content: "Container" } }], icon: { type: "emoji", emoji: "📦" } } },
      ]);
      runJson(`block append ${pageId} --blocks '${calloutBlocks}'`);

      const items = (runJson(`block list ${pageId} --raw`).items as Array<Record<string, unknown>>);
      const paragraph = items.find((b) => b.content === "Standalone text")!;
      const callout = items.find((b) => b.type === "callout")!;

      // Move paragraph into callout
      runJson(`block move ${paragraph.id} --parent ${callout.id}`);

      // Verify: paragraph is gone from top level, callout has children
      const afterItems = (runJson(`block list ${pageId} --raw`).items as Array<Record<string, unknown>>);
      expect(afterItems.length).toBe(1);
      expect(afterItems[0]!.type).toBe("callout");
      expect(afterItems[0]!.hasChildren).toBe(true);
    });
  });

  // --- Page read ---

  describe("page get", () => {
    test("returns page properties", () => {
      const result = runJson(`page get ${pageId}`);
      expect(result.id).toBeDefined();
      expect(result.url).toBeDefined();
    });

    test("returns page with content", () => {
      const result = runJson(`page get ${pageId} --content`);
      expect(result.content).toBeDefined();
    });
  });

  // --- Comment ---

  describe("comment", () => {
    test("adds and lists a page comment", () => {
      const addResult = runJson(`comment page ${pageId} "Integration test comment"`);
      expect(addResult.id).toBeDefined();
      expect(addResult.body).toBe("Integration test comment");

      const listResult = runJson(`comment list ${pageId}`);
      const items = listResult.items as Array<Record<string, unknown>>;
      const found = items.some((c) => c.body === "Integration test comment");
      expect(found).toBe(true);
    });
  });
});
