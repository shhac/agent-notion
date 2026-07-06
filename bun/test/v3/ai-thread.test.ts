import { describe, test, expect } from "bun:test";
import { getThreadContent } from "../../src/notion/v3/ai.ts";
import { normalizeRecordMapResponse } from "../../src/notion/v3/record-map.ts";
import { createMockClient } from "../helpers/mock-v3-client.ts";

/** Role-wrapped wire entry, as syncRecordValues returns it before normalization. */
const wireEntry = (entity: unknown) => ({ value: { value: entity, role: "reader" } });

describe("getThreadContent", () => {
  test("parses thread and messages from role-wrapped sync responses", async () => {
    const thread = {
      id: "thread-1",
      data: { title: "Example Thread" },
      messages: ["msg-1", "msg-2"],
    };
    const userMsg = {
      id: "msg-1",
      created_time: 1700000000000,
      step: { type: "user", value: [["hello there"]] },
    };
    const agentMsg = {
      id: "msg-2",
      created_time: 1700000001000,
      step: { type: "agent-inference", value: [{ type: "text", content: "hi!" }] },
    };

    // The real client normalizes every response in post(); do the same here
    // so the test exercises the seam production data flows through.
    const { client } = createMockClient({
      syncRecordValuesForPointers: (pointers: Array<{ table: string }>) =>
        normalizeRecordMapResponse(
          pointers[0]?.table === "thread"
            ? { recordMap: { thread: { "thread-1": wireEntry(thread) } } }
            : {
                recordMap: {
                  thread_message: {
                    "msg-1": wireEntry(userMsg),
                    "msg-2": wireEntry(agentMsg),
                  },
                },
              },
        ),
    });

    const result = await getThreadContent(client, "thread-1", "space-1");

    expect(result.title).toBe("Example Thread");
    expect(result.messages).toEqual([
      { id: "msg-1", role: "user", content: "hello there", createdAt: 1700000000000 },
      { id: "msg-2", role: "assistant", content: "hi!", createdAt: 1700000001000 },
    ]);
  });

  test("returns a not-found note when the thread record is missing", async () => {
    const { client } = createMockClient();

    const result = await getThreadContent(client, "thread-x", "space-1");

    expect(result.messages).toEqual([]);
    expect(String(result.raw?.["_note"])).toContain("Thread not found");
  });

  test("returns title with no messages when the thread has none", async () => {
    const thread = { id: "thread-1", data: { title: "Empty Thread" }, messages: [] };
    const { client } = createMockClient({
      syncRecordValuesForPointers: () =>
        normalizeRecordMapResponse({
          recordMap: { thread: { "thread-1": wireEntry(thread) } },
        }),
    });

    const result = await getThreadContent(client, "thread-1", "space-1");

    expect(result.title).toBe("Empty Thread");
    expect(result.messages).toEqual([]);
  });
});
