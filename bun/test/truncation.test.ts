import { describe, test, expect, beforeEach } from "bun:test";
import { applyTruncation, configureTruncation } from "../src/lib/truncation.ts";

describe("applyTruncation", () => {
  beforeEach(() => configureTruncation({}));

  test("truncates description over 200 chars", () => {
    const long = "a".repeat(300);
    const result = applyTruncation({ description: long }) as Record<string, unknown>;
    expect(result.description).toBe(`${"a".repeat(200)}\u2026`);
    expect(result.descriptionLength).toBe(300);
  });

  test("truncates body over 200 chars", () => {
    const long = "b".repeat(250);
    const result = applyTruncation({ body: long }) as Record<string, unknown>;
    expect(result.body).toBe(`${"b".repeat(200)}\u2026`);
    expect(result.bodyLength).toBe(250);
  });

  test("truncates content over 200 chars", () => {
    const long = "c".repeat(500);
    const result = applyTruncation({ content: long }) as Record<string, unknown>;
    expect(result.content).toBe(`${"c".repeat(200)}\u2026`);
    expect(result.contentLength).toBe(500);
  });

  test("preserves short description without truncation", () => {
    const result = applyTruncation({ description: "short text" }) as Record<string, unknown>;
    expect(result.description).toBe("short text");
    expect(result.descriptionLength).toBe(10);
  });

  test("preserves field at exactly 200 chars", () => {
    const exact = "x".repeat(200);
    const result = applyTruncation({ description: exact }) as Record<string, unknown>;
    expect(result.description).toBe(exact);
    expect(result.descriptionLength).toBe(200);
  });

  test("does not add companion for non-truncatable fields", () => {
    const result = applyTruncation({ title: "hello", name: "world" }) as Record<string, unknown>;
    expect(result.title).toBe("hello");
    expect(result.titleLength).toBeUndefined();
    expect(result.name).toBe("world");
    expect(result.nameLength).toBeUndefined();
  });

  test("handles nested objects", () => {
    const data = { page: { description: "d".repeat(300), title: "Test" } };
    const result = applyTruncation(data) as Record<string, unknown>;
    const page = result.page as Record<string, unknown>;
    expect(page.description).toBe(`${"d".repeat(200)}\u2026`);
    expect(page.descriptionLength).toBe(300);
    expect(page.title).toBe("Test");
  });

  test("handles arrays with truncatable fields", () => {
    const data = [
      { id: "1", body: "b".repeat(300) },
      { id: "2", body: "short" },
    ];
    const result = applyTruncation(data) as Record<string, unknown>[];
    expect(result[0]!.body).toBe(`${"b".repeat(200)}\u2026`);
    expect(result[0]!.bodyLength).toBe(300);
    expect(result[1]!.body).toBe("short");
    expect(result[1]!.bodyLength).toBe(5);
  });

  test("handles null and undefined values", () => {
    expect(applyTruncation(null)).toBeNull();
    expect(applyTruncation(undefined)).toBeUndefined();
    const result = applyTruncation({ description: null, title: "ok" }) as Record<string, unknown>;
    expect(result.description).toBeNull();
    expect(result.descriptionLength).toBeUndefined();
  });

  test("passes through non-object primitives", () => {
    expect(applyTruncation("hello")).toBe("hello");
    expect(applyTruncation(42)).toBe(42);
    expect(applyTruncation(true)).toBe(true);
  });

  test("preserves non-string truncatable field values", () => {
    const result = applyTruncation({ content: 42 }) as Record<string, unknown>;
    expect(result.content).toBe(42);
    expect(result.contentLength).toBeUndefined();
  });
});

describe("configureTruncation --full", () => {
  test("expands all fields when --full is set", () => {
    configureTruncation({ full: true });
    const long = "a".repeat(300);
    const result = applyTruncation({ description: long, body: long }) as Record<string, unknown>;
    expect(result.description).toBe(long);
    expect(result.descriptionLength).toBe(300);
    expect(result.body).toBe(long);
    expect(result.bodyLength).toBe(300);
  });
});

describe("configureTruncation --expand", () => {
  test("expands only specified fields", () => {
    configureTruncation({ expand: "description" });
    const long = "a".repeat(300);
    const result = applyTruncation({ description: long, body: long }) as Record<string, unknown>;
    expect(result.description).toBe(long);
    expect(result.descriptionLength).toBe(300);
    expect(result.body).toBe(`${"a".repeat(200)}\u2026`);
    expect(result.bodyLength).toBe(300);
  });

  test("expands multiple comma-separated fields", () => {
    configureTruncation({ expand: "description,body" });
    const long = "a".repeat(300);
    const result = applyTruncation({
      description: long,
      body: long,
      content: long,
    }) as Record<string, unknown>;
    expect(result.description).toBe(long);
    expect(result.body).toBe(long);
    expect(result.content).toBe(`${"a".repeat(200)}\u2026`);
  });

  test("handles whitespace in expand list", () => {
    configureTruncation({ expand: " description , body " });
    const long = "a".repeat(300);
    const result = applyTruncation({ description: long, body: long }) as Record<string, unknown>;
    expect(result.description).toBe(long);
    expect(result.body).toBe(long);
  });

  test("--full takes precedence over --expand", () => {
    configureTruncation({ full: true, expand: "description" });
    const long = "a".repeat(300);
    const result = applyTruncation({ body: long }) as Record<string, unknown>;
    expect(result.body).toBe(long);
  });
});

describe("configureTruncation custom maxLength", () => {
  test("uses custom maxLength", () => {
    configureTruncation({ maxLength: 50 });
    const long = "a".repeat(100);
    const result = applyTruncation({ description: long }) as Record<string, unknown>;
    expect(result.description).toBe(`${"a".repeat(50)}\u2026`);
    expect(result.descriptionLength).toBe(100);
  });
});

describe("configureTruncation reset", () => {
  test("reset to default truncation", () => {
    configureTruncation({ full: true });
    configureTruncation({});
    const long = "a".repeat(300);
    const result = applyTruncation({ description: long }) as Record<string, unknown>;
    expect(result.description).toBe(`${"a".repeat(200)}\u2026`);
  });
});
