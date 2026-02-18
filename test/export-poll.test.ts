import { describe, test, expect, mock, beforeEach, afterEach } from "bun:test";
import type { V3ExportTask } from "../src/notion/v3/client.ts";
import { V3HttpError } from "../src/notion/v3/client.ts";

// Mock Bun.write to avoid filesystem writes
const originalBunWrite = Bun.write;
let writeCalls: Array<{ path: string; data: any }> = [];

// Mock fetch to avoid real HTTP
const originalFetch = globalThis.fetch;

// --- Helpers ---

type MockClientOpts = {
  enqueueTask?: () => any;
  getTasks?: (taskIds: string[]) => any;
};

function createMockExportClient(opts: MockClientOpts = {}) {
  return {
    enqueueTask: async (task: any) => opts.enqueueTask?.() ?? { taskId: "task-1" },
    getTasks: async (taskIds: string[]) => opts.getTasks?.(taskIds) ?? { results: [] },
  } as any;
}

// We need to import after mocks are set up but the module uses static imports.
// Since poll.ts uses global `fetch` and `Bun.write`, we intercept them per-test.

import { exportAndDownload, defaultExportFilename } from "../src/cli/export/poll.ts";

describe("defaultExportFilename", () => {
  test("returns a filename with timestamp", () => {
    const name = defaultExportFilename();
    expect(name).toMatch(/^notion-export-\d{4}-\d{2}-\d{2}T\d{2}-\d{2}-\d{2}\.zip$/);
  });
});

describe("exportAndDownload", () => {
  let stderrOutput: string;
  const originalStderrWrite = process.stderr.write;

  beforeEach(() => {
    stderrOutput = "";
    writeCalls = [];
    process.stderr.write = ((chunk: any) => {
      stderrOutput += String(chunk);
      return true;
    }) as any;
  });

  afterEach(() => {
    process.stderr.write = originalStderrWrite;
    globalThis.fetch = originalFetch;
    // @ts-ignore
    Bun.write = originalBunWrite;
  });

  test("completes success flow: enqueue → poll → download", async () => {
    let pollCount = 0;
    const client = createMockExportClient({
      enqueueTask: () => ({ taskId: "task-1" }),
      getTasks: () => {
        pollCount++;
        if (pollCount < 2) {
          return {
            results: [{
              id: "task-1",
              eventName: "exportBlock",
              state: "in_progress" as const,
              status: { pagesExported: 5 },
            }],
          };
        }
        return {
          results: [{
            id: "task-1",
            eventName: "exportBlock",
            state: "success" as const,
            status: { pagesExported: 10, exportURL: "https://example.com/export.zip" },
          }],
        };
      },
    });

    // Mock fetch for download
    globalThis.fetch = (async (url: any) => ({
      ok: true,
      status: 200,
      statusText: "OK",
      arrayBuffer: async () => new ArrayBuffer(8),
    })) as any;

    // Mock Bun.write
    // @ts-ignore
    Bun.write = async (path: any, data: any) => {
      writeCalls.push({ path: String(path), data });
    };

    const result = await exportAndDownload(
      client,
      { eventName: "exportBlock", request: { blockId: "page-1" } },
      "output.zip",
      { pollInterval: 10, timeout: 5000 },
    );

    expect(result.pagesExported).toBe(10);
    expect(result.path).toContain("output.zip");
    expect(writeCalls).toHaveLength(1);
    expect(stderrOutput).toContain("Export task queued");
  });

  test("throws on export failure", async () => {
    const client = createMockExportClient({
      enqueueTask: () => ({ taskId: "task-1" }),
      getTasks: () => ({
        results: [{
          id: "task-1",
          eventName: "exportBlock",
          state: "failure" as const,
          error: "Something went wrong",
        }],
      }),
    });

    await expect(
      exportAndDownload(
        client,
        { eventName: "exportBlock", request: {} },
        "out.zip",
        { pollInterval: 10, timeout: 5000 },
      ),
    ).rejects.toThrow(/Export failed.*Something went wrong/);
  });

  test("throws on missing download URL", async () => {
    const client = createMockExportClient({
      enqueueTask: () => ({ taskId: "task-1" }),
      getTasks: () => ({
        results: [{
          id: "task-1",
          eventName: "exportBlock",
          state: "success" as const,
          status: { pagesExported: 10 },
          // no exportURL
        }],
      }),
    });

    await expect(
      exportAndDownload(
        client,
        { eventName: "exportBlock", request: {} },
        "out.zip",
        { pollInterval: 10, timeout: 5000 },
      ),
    ).rejects.toThrow(/no download URL/);
  });

  test("throws on download HTTP error", async () => {
    const client = createMockExportClient({
      enqueueTask: () => ({ taskId: "task-1" }),
      getTasks: () => ({
        results: [{
          id: "task-1",
          eventName: "exportBlock",
          state: "success" as const,
          status: { exportURL: "https://example.com/export.zip", pagesExported: 5 },
        }],
      }),
    });

    globalThis.fetch = (async () => ({
      ok: false,
      status: 403,
      statusText: "Forbidden",
    })) as any;

    await expect(
      exportAndDownload(
        client,
        { eventName: "exportBlock", request: {} },
        "out.zip",
        { pollInterval: 10, timeout: 5000 },
      ),
    ).rejects.toThrow(/Download failed.*403/);
  });

  test("throws on poll timeout", async () => {
    const client = createMockExportClient({
      enqueueTask: () => ({ taskId: "task-1" }),
      getTasks: () => ({
        results: [{
          id: "task-1",
          eventName: "exportBlock",
          state: "in_progress" as const,
          status: { pagesExported: 1 },
        }],
      }),
    });

    await expect(
      exportAndDownload(
        client,
        { eventName: "exportBlock", request: {} },
        "out.zip",
        { pollInterval: 10, timeout: 50 },
      ),
    ).rejects.toThrow(/timed out/);
  });

  test("throws when task not found in response", async () => {
    const client = createMockExportClient({
      enqueueTask: () => ({ taskId: "task-1" }),
      getTasks: () => ({ results: [undefined] }),
    });

    await expect(
      exportAndDownload(
        client,
        { eventName: "exportBlock", request: {} },
        "out.zip",
        { pollInterval: 10, timeout: 5000 },
      ),
    ).rejects.toThrow(/not found/);
  });

  test("retries on transient server errors", async () => {
    let callCount = 0;
    const client = createMockExportClient({
      enqueueTask: () => ({ taskId: "task-1" }),
      getTasks: () => {
        callCount++;
        if (callCount === 1) {
          throw new V3HttpError("Server error", 500, "getTasks");
        }
        return {
          results: [{
            id: "task-1",
            eventName: "exportBlock",
            state: "success" as const,
            status: { exportURL: "https://example.com/export.zip", pagesExported: 3 },
          }],
        };
      },
    });

    globalThis.fetch = (async () => ({
      ok: true,
      status: 200,
      arrayBuffer: async () => new ArrayBuffer(4),
    })) as any;

    // @ts-ignore
    Bun.write = async () => {};

    const result = await exportAndDownload(
      client,
      { eventName: "exportBlock", request: {} },
      "out.zip",
      { pollInterval: 10, timeout: 10000 },
    );

    expect(result.pagesExported).toBe(3);
    expect(stderrOutput).toContain("retrying");
  });
});
