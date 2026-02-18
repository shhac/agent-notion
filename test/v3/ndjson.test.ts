import { describe, test, expect } from "bun:test";
import { parseNdjson } from "../../src/notion/v3/ndjson.ts";

// --- Helpers ---

function mockResponse(lines: string[]): Response {
  const text = lines.join("\n");
  return new Response(text, {
    headers: { "content-type": "application/x-ndjson" },
  });
}

async function collect<T>(iter: AsyncIterable<T>): Promise<T[]> {
  const results: T[] = [];
  for await (const item of iter) results.push(item);
  return results;
}

// ===================================================================
// parseNdjson
// ===================================================================

describe("parseNdjson", () => {
  test("parses single JSON line", async () => {
    const resp = mockResponse(['{"type":"test","id":1}']);
    const results = await collect(parseNdjson(resp));
    expect(results).toEqual([{ type: "test", id: 1 }]);
  });

  test("parses multiple lines", async () => {
    const resp = mockResponse([
      '{"type":"a"}',
      '{"type":"b"}',
      '{"type":"c"}',
    ]);
    const results = await collect(parseNdjson(resp));
    expect(results).toHaveLength(3);
    expect(results[0]).toEqual({ type: "a" });
    expect(results[2]).toEqual({ type: "c" });
  });

  test("skips empty lines", async () => {
    const resp = mockResponse(['{"type":"a"}', "", "", '{"type":"b"}']);
    const results = await collect(parseNdjson(resp));
    expect(results).toHaveLength(2);
  });

  test("skips malformed JSON lines (no throw)", async () => {
    const resp = mockResponse([
      '{"type":"a"}',
      "not valid json",
      '{"type":"b"}',
    ]);
    const results = await collect(parseNdjson(resp));
    expect(results).toHaveLength(2);
    expect(results[0]).toEqual({ type: "a" });
    expect(results[1]).toEqual({ type: "b" });
  });

  test("handles trailing content without newline", async () => {
    // No trailing newline — last item should still be flushed
    const text = '{"type":"first"}\n{"type":"last"}';
    const resp = new Response(text, {
      headers: { "content-type": "application/x-ndjson" },
    });
    const results = await collect(parseNdjson(resp));
    expect(results).toHaveLength(2);
    expect(results[1]).toEqual({ type: "last" });
  });

  test("returns empty for response with no body", async () => {
    const resp = new Response(null, {
      headers: { "content-type": "application/x-ndjson" },
    });
    const results = await collect(parseNdjson(resp));
    expect(results).toHaveLength(0);
  });

  test("calls onRawLine callback for each line", async () => {
    const lines: string[] = [];
    const resp = mockResponse(['{"type":"a"}', '{"type":"b"}']);
    await collect(parseNdjson(resp, (line) => lines.push(line)));
    expect(lines).toEqual(['{"type":"a"}', '{"type":"b"}']);
  });
});
