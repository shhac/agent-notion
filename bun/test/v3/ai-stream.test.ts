import { describe, test, expect } from "bun:test";
import type { NdjsonEvent } from "../../src/notion/v3/ai-types.ts";
import {
  normalizePatchStream,
  processInferenceStream,
} from "../../src/notion/v3/ai.ts";

// --- Helpers ---

async function* toAsync(
  events: NdjsonEvent[],
): AsyncIterable<NdjsonEvent> {
  for (const e of events) yield e;
}

async function collect(iter: AsyncIterable<NdjsonEvent>): Promise<NdjsonEvent[]> {
  const results: NdjsonEvent[] = [];
  for await (const item of iter) results.push(item);
  return results;
}

// ===================================================================
// normalizePatchStream
// ===================================================================

describe("normalizePatchStream", () => {
  test("passes through standard agent-inference events unchanged", async () => {
    const event: NdjsonEvent = {
      type: "agent-inference",
      id: "inf-1",
      value: [{ type: "text", content: "Hello" }],
      traceId: "t1",
      startedAt: 1,
      previousAttemptValues: [],
    };
    const results = await collect(normalizePatchStream(toAsync([event])));
    expect(results).toHaveLength(1);
    expect(results[0]).toEqual(event);
  });

  test("passes through record-map events", async () => {
    const event: NdjsonEvent = { type: "record-map" };
    const results = await collect(normalizePatchStream(toAsync([event])));
    expect(results).toHaveLength(1);
    expect(results[0]).toEqual(event);
  });

  test("converts patch-start + patch sequence into agent-inference events", async () => {
    const events: NdjsonEvent[] = [
      {
        type: "patch-start",
        data: { s: [] },
        version: 1,
      },
      {
        type: "patch",
        v: [
          {
            o: "a",
            p: "/s/-",
            v: {
              type: "agent-inference",
              id: "inf-1",
              value: [{ type: "text", content: "Hi" }],
              traceId: "t1",
              startedAt: 1,
              previousAttemptValues: [],
            },
          },
        ],
      },
    ];
    const results = await collect(normalizePatchStream(toAsync(events)));
    expect(results).toHaveLength(1);
    expect(results[0]!.type).toBe("agent-inference");
  });

  test("handles text append (o: x) correctly", async () => {
    const events: NdjsonEvent[] = [
      {
        type: "patch-start",
        data: {
          s: [
            {
              type: "agent-inference",
              id: "inf-1",
              value: [{ type: "text", content: "Hel" }],
              traceId: "t1",
              startedAt: 1,
              previousAttemptValues: [],
            },
          ],
        },
        version: 1,
      },
      {
        type: "patch",
        v: [{ o: "x", p: "/s/0/value/0/content", v: "lo" }],
      },
    ];
    const results = await collect(normalizePatchStream(toAsync(events)));
    expect(results).toHaveLength(1);
    const inf = results[0] as any;
    expect(inf.value[0].content).toBe("Hello");
  });

  test("handles remove (o: r) correctly", async () => {
    const events: NdjsonEvent[] = [
      {
        type: "patch-start",
        data: {
          s: [
            {
              type: "agent-inference",
              id: "inf-1",
              value: [
                { type: "text", content: "keep" },
                { type: "thinking", content: "remove" },
              ],
              traceId: "t1",
              startedAt: 1,
              previousAttemptValues: [],
            },
          ],
        },
        version: 1,
      },
      {
        type: "patch",
        v: [{ o: "r", p: "/s/0/value/1" }],
      },
    ];
    const results = await collect(normalizePatchStream(toAsync(events)));
    expect(results).toHaveLength(1);
    const inf = results[0] as any;
    expect(inf.value).toHaveLength(1);
    expect(inf.value[0].content).toBe("keep");
  });

  test("handles add (o: a) for new slots via /s/-", async () => {
    const events: NdjsonEvent[] = [
      { type: "patch-start", data: { s: [] }, version: 1 },
      {
        type: "patch",
        v: [
          {
            o: "a",
            p: "/s/-",
            v: {
              type: "agent-inference",
              id: "inf-1",
              value: [{ type: "text", content: "new slot" }],
              traceId: "t1",
              startedAt: 1,
              previousAttemptValues: [],
            },
          },
        ],
      },
    ];
    const results = await collect(normalizePatchStream(toAsync(events)));
    expect(results).toHaveLength(1);
    const inf = results[0] as any;
    expect(inf.value[0].content).toBe("new slot");
  });
});

// ===================================================================
// processInferenceStream
// ===================================================================

describe("processInferenceStream", () => {
  test("extracts response text from agent-inference events", async () => {
    const events: NdjsonEvent[] = [
      {
        type: "agent-inference",
        id: "inf-1",
        value: [{ type: "text", content: "Hello world" }],
        traceId: "t1",
        startedAt: 1,
        previousAttemptValues: [],
      },
    ];
    const result = await processInferenceStream(toAsync(events));
    expect(result.response).toBe("Hello world");
  });

  test("extracts title from title events", async () => {
    const events: NdjsonEvent[] = [
      { type: "title", value: "My Chat Title" },
      {
        type: "agent-inference",
        id: "inf-1",
        value: [{ type: "text", content: "response" }],
        traceId: "t1",
        startedAt: 1,
        previousAttemptValues: [],
      },
    ];
    const result = await processInferenceStream(toAsync(events));
    expect(result.title).toBe("My Chat Title");
  });

  test("extracts token counts from final agent-inference event", async () => {
    const events: NdjsonEvent[] = [
      {
        type: "agent-inference",
        id: "inf-1",
        value: [{ type: "text", content: "response" }],
        traceId: "t1",
        startedAt: 1,
        previousAttemptValues: [],
        finishedAt: 2,
        inputTokens: 100,
        outputTokens: 50,
        cachedTokensRead: 10,
        model: "oatmeal-cookie",
      },
    ];
    const result = await processInferenceStream(toAsync(events));
    expect(result.tokens).toEqual({ input: 100, output: 50, cached: 10 });
    expect(result.model).toBe("oatmeal-cookie");
  });

  test("strips lang tag from response", async () => {
    const events: NdjsonEvent[] = [
      {
        type: "agent-inference",
        id: "inf-1",
        value: [{ type: "text", content: '<lang primary="en-US"/>Hello' }],
        traceId: "t1",
        startedAt: 1,
        previousAttemptValues: [],
      },
    ];
    const result = await processInferenceStream(toAsync(events));
    expect(result.response).toBe("Hello");
  });

  test("calls onStreamChunk for streaming", async () => {
    const chunks: string[] = [];
    const events: NdjsonEvent[] = [
      {
        type: "agent-inference",
        id: "inf-1",
        value: [{ type: "text", content: "Hello" }],
        traceId: "t1",
        startedAt: 1,
        previousAttemptValues: [],
      },
      {
        type: "agent-inference",
        id: "inf-1",
        value: [{ type: "text", content: "Hello world" }],
        traceId: "t1",
        startedAt: 1,
        previousAttemptValues: [],
      },
    ];
    await processInferenceStream(toAsync(events), (chunk) => chunks.push(chunk));
    expect(chunks).toEqual(["Hello", " world"]);
  });

  test("handles empty event stream", async () => {
    const result = await processInferenceStream(toAsync([]));
    expect(result.response).toBe("");
    expect(result.title).toBeUndefined();
    expect(result.tokens).toBeUndefined();
  });
});
