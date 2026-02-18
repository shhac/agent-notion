import { describe, expect, test } from "bun:test";
import { execSync } from "node:child_process";

function getUsageOutput(command: string): string {
  return execSync(`bun run src/index.ts ${command}`, {
    encoding: "utf8",
    timeout: 10000,
  }).trim();
}

function estimateTokens(text: string): number {
  return Math.ceil(text.length / 4);
}

describe("usage token budgets", () => {
  test("top-level usage < 1100 tokens", () => {
    const output = getUsageOutput("usage");
    const tokens = estimateTokens(output);
    expect(tokens).toBeLessThan(1100);
    expect(output.length).toBeLessThan(4400); // chars/4 estimate
  });

  test("search usage < 500 tokens", () => {
    const output = getUsageOutput("search usage");
    const tokens = estimateTokens(output);
    expect(tokens).toBeLessThan(500);
  });

  test("database usage < 500 tokens", () => {
    const output = getUsageOutput("database usage");
    const tokens = estimateTokens(output);
    expect(tokens).toBeLessThan(500);
  });

  test("page usage < 500 tokens", () => {
    const output = getUsageOutput("page usage");
    const tokens = estimateTokens(output);
    expect(tokens).toBeLessThan(500);
  });

  test("block usage < 500 tokens", () => {
    const output = getUsageOutput("block usage");
    const tokens = estimateTokens(output);
    expect(tokens).toBeLessThan(500);
  });

  test("comment usage < 500 tokens", () => {
    const output = getUsageOutput("comment usage");
    const tokens = estimateTokens(output);
    expect(tokens).toBeLessThan(500);
  });

  test("user usage < 500 tokens", () => {
    const output = getUsageOutput("user usage");
    const tokens = estimateTokens(output);
    expect(tokens).toBeLessThan(500);
  });

  test("config usage < 500 tokens", () => {
    const output = getUsageOutput("config usage");
    const tokens = estimateTokens(output);
    expect(tokens).toBeLessThan(500);
  });
});
