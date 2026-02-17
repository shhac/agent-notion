import { describe, expect, test } from "bun:test";
import { normalizeId } from "../src/lib/ids.ts";

describe("normalizeId", () => {
  test("adds dashes to 32-char hex string", () => {
    expect(normalizeId("30a61d9c1112802f95fef30d3a601ec5")).toBe(
      "30a61d9c-1112-802f-95fe-f30d3a601ec5",
    );
  });

  test("passes through already-dashed UUID", () => {
    expect(normalizeId("30a61d9c-1112-802f-95fe-f30d3a601ec5")).toBe(
      "30a61d9c-1112-802f-95fe-f30d3a601ec5",
    );
  });

  test("handles uppercase hex", () => {
    expect(normalizeId("30A61D9C1112802F95FEF30D3A601EC5")).toBe(
      "30a61d9c-1112-802f-95fe-f30d3a601ec5",
    );
  });

  test("returns non-UUID strings as-is", () => {
    expect(normalizeId("not-a-uuid")).toBe("not-a-uuid");
    expect(normalizeId("")).toBe("");
    expect(normalizeId("12345")).toBe("12345");
  });

  test("returns strings with non-hex chars as-is", () => {
    expect(normalizeId("30a61d9c1112802f95fef30d3a601ecZ")).toBe(
      "30a61d9c1112802f95fef30d3a601ecZ",
    );
  });
});
