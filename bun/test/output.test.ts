import { describe, expect, test } from "bun:test";
import { pruneEmpty } from "../src/lib/output.ts";

const prune = (v: unknown) => pruneEmpty(v);

describe("pruneEmpty", () => {
  test("removes null and undefined values", () => {
    expect(prune({ a: 1, b: null, c: undefined })).toEqual({ a: 1 });
  });

  test("removes empty strings", () => {
    expect(prune({ a: "hello", b: "", c: "  " })).toEqual({ a: "hello" });
  });

  test("preserves zero and false", () => {
    expect(prune({ a: 0, b: false, c: true })).toEqual({ a: 0, b: false, c: true });
  });

  test("removes empty arrays", () => {
    expect(prune({ a: [1, 2], b: [] })).toEqual({ a: [1, 2] });
  });

  test("prunes nested objects recursively", () => {
    expect(prune({ a: { b: null, c: { d: "" } }, e: "keep" })).toEqual({ e: "keep" });
  });

  test("prunes null entries from arrays", () => {
    expect(prune([1, null, "hello", undefined, ""])).toEqual([1, "hello"]);
  });

  test("returns empty object for fully pruned input", () => {
    expect(prune({ a: null, b: "" })).toEqual({});
  });

  test("handles deeply nested structures", () => {
    const input = {
      page: {
        id: "abc",
        title: "Test",
        parent: null,
        properties: [
          { name: "Status", value: "Done", extra: null },
          { name: "", value: "", extra: null },
        ],
      },
    };
    expect(prune(input)).toEqual({
      page: {
        id: "abc",
        title: "Test",
        properties: [{ name: "Status", value: "Done" }],
      },
    });
  });
});
