import { describe, test, expect } from "bun:test";
import type { AiModel } from "../../src/notion/v3/ai-types.ts";
import {
  stripLangTag,
  isIncompleteLangTag,
  extractRichText,
  parseThreadMessage,
  resolveModel,
  applyPatchOp,
} from "../../src/notion/v3/ai.ts";
import { CliError } from "../../src/lib/errors.ts";

// ===================================================================
// stripLangTag
// ===================================================================

describe("stripLangTag", () => {
  test("strips complete lang tag", () => {
    expect(stripLangTag('<lang primary="en-US"/> Hello')).toBe("Hello");
  });

  test("strips tag with extra attributes", () => {
    expect(stripLangTag('<lang primary="en-US" secondary="fr"/>text')).toBe(
      "text",
    );
  });

  test("no-op on content without tag", () => {
    expect(stripLangTag("Hello world")).toBe("Hello world");
  });

  test("no-op on empty string", () => {
    expect(stripLangTag("")).toBe("");
  });
});

// ===================================================================
// isIncompleteLangTag
// ===================================================================

describe("isIncompleteLangTag", () => {
  test("returns true for incomplete tag (no closing >)", () => {
    expect(isIncompleteLangTag("<lang primary")).toBe(true);
  });

  test("returns false for complete tag", () => {
    expect(isIncompleteLangTag('<lang primary="en-US"/>')).toBe(false);
  });

  test("returns false for non-tag content", () => {
    expect(isIncompleteLangTag("Hello world")).toBe(false);
  });
});

// ===================================================================
// extractRichText
// ===================================================================

describe("extractRichText", () => {
  test("extracts from single segment", () => {
    expect(extractRichText([["hello"]])).toBe("hello");
  });

  test("extracts from multiple segments", () => {
    expect(extractRichText([["hello"], ["world"]])).toBe("helloworld");
  });

  test("returns string as-is if input is string", () => {
    expect(extractRichText("direct text")).toBe("direct text");
  });

  test("returns undefined for null", () => {
    expect(extractRichText(null)).toBeUndefined();
  });

  test("returns undefined for undefined", () => {
    expect(extractRichText(undefined)).toBeUndefined();
  });

  test("returns undefined for empty array", () => {
    expect(extractRichText([])).toBeUndefined();
  });
});

// ===================================================================
// parseThreadMessage
// ===================================================================

describe("parseThreadMessage", () => {
  test("parses user message", () => {
    const result = parseThreadMessage(
      "msg-1",
      "user",
      { type: "user", value: [["Hello"]] },
      { created_time: 1700000000000 },
    );
    expect(result).toEqual({
      id: "msg-1",
      role: "user",
      content: "Hello",
      createdAt: 1700000000000,
    });
  });

  test("parses agent-inference message with lang tag stripped", () => {
    const result = parseThreadMessage(
      "msg-2",
      "agent-inference",
      {
        type: "agent-inference",
        value: [{ type: "text", content: '<lang primary="en-US"/>response text' }],
      },
      { created_time: 1700000000000 },
    );
    expect(result).toEqual({
      id: "msg-2",
      role: "assistant",
      content: "response text",
      createdAt: 1700000000000,
    });
  });

  test("parses agent-tool-result success", () => {
    const result = parseThreadMessage(
      "msg-3",
      "agent-tool-result",
      { type: "agent-tool-result", toolName: "view", state: "applied" },
      { created_time: 1700000000000 },
    );
    expect(result).toEqual({
      id: "msg-3",
      role: "tool",
      content: 'Tool "view" completed',
      createdAt: 1700000000000,
      toolName: "view",
      toolState: "applied",
    });
  });

  test("parses agent-tool-result error", () => {
    const result = parseThreadMessage(
      "msg-4",
      "agent-tool-result",
      {
        type: "agent-tool-result",
        toolName: "view",
        state: "applied:error",
        error: "Not found",
      },
      { created_time: 1700000000000 },
    );
    expect(result).toEqual({
      id: "msg-4",
      role: "tool",
      content: 'Tool "view" failed: Not found',
      createdAt: 1700000000000,
      toolName: "view",
      toolState: "applied:error",
    });
  });

  test("returns null for config step type", () => {
    expect(
      parseThreadMessage("msg-5", "config", { type: "config" }, {}),
    ).toBeNull();
  });

  test("returns null for context step type", () => {
    expect(
      parseThreadMessage("msg-6", "context", { type: "context" }, {}),
    ).toBeNull();
  });

  test("returns null for title step type", () => {
    expect(
      parseThreadMessage("msg-7", "title", { type: "title" }, {}),
    ).toBeNull();
  });

  test("returns null for unknown step type", () => {
    expect(
      parseThreadMessage("msg-8", "something-else", { type: "something-else" }, {}),
    ).toBeNull();
  });
});

// ===================================================================
// resolveModel
// ===================================================================

describe("resolveModel", () => {
  const models: AiModel[] = [
    {
      model: "oatmeal-cookie",
      modelMessage: "GPT-5.2",
      modelFamily: "openai",
      displayGroup: "intelligent",
    },
    {
      model: "cinnamon-roll",
      modelMessage: "Claude 4 Sonnet",
      modelFamily: "anthropic",
      displayGroup: "fast",
    },
  ];

  test("exact codename match", async () => {
    expect(await resolveModel(models, "oatmeal-cookie", undefined)).toBe(
      "oatmeal-cookie",
    );
  });

  test("case-insensitive display name match", async () => {
    expect(await resolveModel(models, "gpt-5.2", undefined)).toBe(
      "oatmeal-cookie",
    );
  });

  test("partial display name match", async () => {
    expect(await resolveModel(models, "sonnet", undefined)).toBe(
      "cinnamon-roll",
    );
  });

  test("throws CliError for unknown model", async () => {
    await expect(resolveModel(models, "nonexistent", undefined)).rejects.toBeInstanceOf(CliError);
  });

  test("returns undefined when no input provided", async () => {
    expect(await resolveModel(models, undefined, undefined)).toBeUndefined();
  });

  test("falls back to configDefault", async () => {
    expect(await resolveModel(models, undefined, "cinnamon-roll")).toBe(
      "cinnamon-roll",
    );
  });
});

// ===================================================================
// applyPatchOp
// ===================================================================

describe("applyPatchOp", () => {
  test('"a" adds to object', () => {
    const slots: Array<Record<string, unknown>> = [{ type: "test" }];
    applyPatchOp(slots, "a", "/s/0/key", "value");
    expect(slots[0]!.key).toBe("value");
  });

  test('"a" appends to array with -', () => {
    const slots: Array<Record<string, unknown>> = [{ arr: ["a", "b"] }];
    applyPatchOp(slots, "a", "/s/0/arr/-", "c");
    expect(slots[0]!.arr).toEqual(["a", "b", "c"]);
  });

  test('"x" appends to string', () => {
    const slots: Array<Record<string, unknown>> = [{ content: "Hello" }];
    applyPatchOp(slots, "x", "/s/0/content", " World");
    expect(slots[0]!.content).toBe("Hello World");
  });

  test('"r" removes from array (splice)', () => {
    const slots: Array<Record<string, unknown>> = [{ arr: ["a", "b", "c"] }];
    applyPatchOp(slots, "r", "/s/0/arr/1", undefined);
    expect(slots[0]!.arr).toEqual(["a", "c"]);
  });

  test('"r" deletes from object', () => {
    const slots: Array<Record<string, unknown>> = [{ key: "val", other: 1 }];
    applyPatchOp(slots, "r", "/s/0/key", undefined);
    expect(slots[0]!.key).toBeUndefined();
    expect(slots[0]!.other).toBe(1);
  });

  test("nested paths work", () => {
    const slots: Array<Record<string, unknown>> = [
      { value: [{ content: "start" }] },
    ];
    applyPatchOp(slots, "x", "/s/0/value/0/content", " end");
    expect((slots[0]!.value as any[])[0].content).toBe("start end");
  });

  test("no-op for invalid slot index", () => {
    const slots: Array<Record<string, unknown>> = [{ type: "test" }];
    applyPatchOp(slots, "a", "/s/5/key", "value");
    expect(slots).toHaveLength(1);
    expect(slots[0]!).toEqual({ type: "test" });
  });

  test("no-op for path not starting with /s", () => {
    const slots: Array<Record<string, unknown>> = [{ type: "test" }];
    applyPatchOp(slots, "a", "/x/0/key", "value");
    expect(slots[0]!).toEqual({ type: "test" });
  });

  test('"a" with /s/- appends new slot', () => {
    const slots: Array<Record<string, unknown>> = [];
    applyPatchOp(slots, "a", "/s/-", { type: "agent-inference", value: [] });
    expect(slots).toHaveLength(1);
    expect(slots[0]!.type).toBe("agent-inference");
  });
});
